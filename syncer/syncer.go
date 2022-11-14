package syncer

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

// SyncDirection indicates which direction data is flowing for this particular syncer
type SyncDirection string

const (
	resyncPeriod       = 10 * time.Hour
	syncerApplyManager = "syncer"

	// SyncDown indicates a syncer watches resources on the global hub and applies the spec to the leaf cluster
	SyncDown SyncDirection = "down"

	// SyncUp indicates a syncer watches resources on the leaft cluster and applies the status to the global hub
	SyncUp SyncDirection = "up"
)

// SyncerConfig defines the syncer configuration that is guaranteed to
// vary across syncer deployments.
type SyncerConfig struct {
	UpstreamConfig   *rest.Config
	DownstreamConfig *rest.Config
}

func StartSyncer(ctx context.Context, cfg *SyncerConfig, numSyncerThreads int) error {

	klog.Infof("Creating spec syncer")
	specSyncer, err := NewSpecSyncer(cfg.UpstreamConfig, cfg.DownstreamConfig)
	if err != nil {
		return err
	}

	klog.Infof("Creating status syncer")
	statusSyncer, err := NewStatusSyncer(cfg.DownstreamConfig, cfg.UpstreamConfig)
	if err != nil {
		return err
	}

	go specSyncer.Start(ctx, numSyncerThreads)
	go statusSyncer.Start(ctx, numSyncerThreads)

	return nil
}

type UpsertFunc func(ctx context.Context, gvr schema.GroupVersionResource, namespace string, unstrob *unstructured.Unstructured) error
type DeleteFunc func(ctx context.Context, gvr schema.GroupVersionResource, namespace, name string) error

type Controller struct {
	name  string
	queue workqueue.RateLimitingInterface

	fromInformers dynamicinformer.DynamicSharedInformerFactory
	fromConfig    *rest.Config
	toClient      dynamic.Interface

	upsertFn  UpsertFunc
	deleteFn  DeleteFunc
	direction SyncDirection

	gvrs []string
}

// New returns a new syncer Controller syncing spec from "from" to "to".
func New(fromClient, toClient dynamic.Interface, fromConfig *rest.Config, direction SyncDirection) (*Controller, error) {
	controllerName := string(direction) + "--regional-hub-->global-hub"
	if direction == SyncDown {
		controllerName = string(direction) + "--global-hub-->regional-hub"
	}
	queue := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "globalhub-"+controllerName)

	c := Controller{
		name:       controllerName,
		queue:      queue,
		toClient:   toClient,
		fromConfig: fromConfig,
		direction:  direction,
	}

	if direction == SyncDown {
		c.upsertFn = c.applyToDownstream
		c.deleteFn = c.deleteFromDownstream
		c.gvrs = []string{
			"policies.v1.policy.open-cluster-management.io",
			"placementbindings.v1.policy.open-cluster-management.io",
			"placementrules.v1.apps.open-cluster-management.io",
		}
	} else {
		c.upsertFn = c.updateStatusInUpstream
		c.gvrs = []string{
			"policies.v1.policy.open-cluster-management.io",
			"managedclusters.v1.cluster.open-cluster-management.io",
			"clustermanagementaddons.v1alpha1.addon.open-cluster-management.io",
			"customresourcedefinitions.v1.apiextensions.k8s.io",
		}
	}

	fromInformers := dynamicinformer.NewFilteredDynamicSharedInformerFactory(fromClient, resyncPeriod,
		metav1.NamespaceAll, func(o *metav1.ListOptions) {
			o.LabelSelector = fmt.Sprintf("!%s", "policy.open-cluster-management.io/root-policy")
		})

	// toInformers := dynamicinformer.NewFilteredDynamicSharedInformerFactory(toClient, resyncPeriod,
	// 	metav1.NamespaceAll, func(o *metav1.ListOptions) {})

	for _, gvrstr := range c.gvrs {
		gvr, _ := schema.ParseResourceArg(gvrstr)

		fromInformers.ForResource(*gvr).Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				shouldEnqueue := true
				if c.direction == SyncUp {
					// check the managedcluster CRD to make sure this is a hub controlplane
					if gvr.Resource == "customresourcedefinitions" {
						unstrob, ok := obj.(*unstructured.Unstructured)
						if !ok {
							shouldEnqueue = false
						}
						if unstrob.GetName() != "managedclusters.cluster.open-cluster-management.io" {
							shouldEnqueue = false
						}
					}
				}

				if shouldEnqueue {
					c.AddToQueue(*gvr, obj)
				}
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				shouldEnqueue := true
				if c.direction == SyncUp {
					// check the managedcluster CRD to make sure this is a hub controlplane
					if gvr.Resource == "customresourcedefinitions" {
						unstrob, ok := newObj.(*unstructured.Unstructured)
						if !ok {
							shouldEnqueue = false
						}
						if unstrob.GetName() != "managedclusters.cluster.open-cluster-management.io" {
							shouldEnqueue = false
						}
					}
				}

				if shouldEnqueue {
					if c.direction == SyncDown {
						if !deepEqualApartFromStatus(oldObj, newObj) {
							c.AddToQueue(*gvr, newObj)
						}
					} else {
						if !deepEqualStatus(oldObj, newObj) {
							c.AddToQueue(*gvr, newObj)
						}
					}
				}
			},
			DeleteFunc: func(obj interface{}) {
				shouldEnqueue := true
				if c.direction == SyncUp {
					// check the managedcluster CRD to make sure this is a hub controlplane
					if gvr.Resource == "customresourcedefinitions" {
						unstrob, ok := obj.(*unstructured.Unstructured)
						if !ok {
							shouldEnqueue = false
						}
						if unstrob.GetName() != "managedclusters.cluster.open-cluster-management.io" {
							shouldEnqueue = false
						}
					}
				}

				if shouldEnqueue {
					c.AddToQueue(*gvr, obj)
				}
			},
		})

		klog.InfoS("Set up informer", "direction", c.direction, "gvr", gvr)
	}

	c.fromInformers = fromInformers

	return &c, nil
}

type holder struct {
	gvr       schema.GroupVersionResource
	namespace string
	name      string
}

func (c *Controller) AddToQueue(gvr schema.GroupVersionResource, obj interface{}) {
	objToCheck := obj

	tombstone, ok := objToCheck.(cache.DeletedFinalStateUnknown)
	if ok {
		objToCheck = tombstone.Obj
	}

	metaObj, err := meta.Accessor(objToCheck)
	if err != nil {
		klog.Errorf("%s: error getting meta for %T", c.name, obj)
		return
	}

	qualifiedName := metaObj.GetName()
	if len(metaObj.GetNamespace()) > 0 {
		qualifiedName = metaObj.GetNamespace() + "/" + qualifiedName
	}

	klog.Infof("Syncer %s: adding %s %s to queue", c.name, gvr, qualifiedName)

	c.queue.Add(
		holder{
			gvr:       gvr,
			namespace: metaObj.GetNamespace(),
			name:      metaObj.GetName(),
		},
	)
}

// Start starts N worker processes processing work items.
func (c *Controller) Start(ctx context.Context, numThreads int) {
	defer runtime.HandleCrash()
	defer c.queue.ShutDown()

	c.fromInformers.Start(ctx.Done())
	c.fromInformers.WaitForCacheSync(ctx.Done())

	klog.InfoS("Starting syncer workers", "controller", c.name)
	defer klog.InfoS("Stopping syncer workers", "controller", c.name)
	for i := 0; i < numThreads; i++ {
		go wait.UntilWithContext(ctx, c.startWorker, time.Second)
	}

	<-ctx.Done()
}

// startWorker processes work items until stopCh is closed.
func (c *Controller) startWorker(ctx context.Context) {
	for c.processNextWorkItem(ctx) {
	}
}

func (c *Controller) processNextWorkItem(ctx context.Context) bool {
	// Wait until there is a new item in the working queue
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	h := key.(holder)

	// No matter what, tell the queue we're done with this key, to unblock
	// other workers.
	defer c.queue.Done(key)

	if err := c.process(ctx, h); err != nil {
		runtime.HandleError(fmt.Errorf("syncer %q failed to sync %q, err: %w", c.name, key, err))
		c.queue.AddRateLimited(key)
		return true
	}

	c.queue.Forget(key)

	return true
}

func (c *Controller) process(ctx context.Context, h holder) error {
	klog.V(2).InfoS("Processing", "gvr", h.gvr, "namespace", h.namespace, "name", h.name)

	key := h.name

	if len(h.namespace) > 0 {
		key = h.namespace + "/" + h.name
	}

	obj, exists, err := c.fromInformers.ForResource(h.gvr).Informer().GetIndexer().GetByKey(key)
	if err != nil {
		return err
	}

	if c.direction == SyncDown && !exists {
		klog.InfoS("Object doesn't exist:", "direction", c.direction, "namespace", h.namespace, "name", h.name)
		if c.deleteFn != nil {
			return c.deleteFn(ctx, h.gvr, h.namespace, h.name)
		}
		return nil
	}

	unstrob, isUnstructured := obj.(*unstructured.Unstructured)
	if !isUnstructured {
		return fmt.Errorf("%s: object to synchronize is expected to be Unstructured, but is %T", c.name, obj)
	}

	if c.upsertFn != nil {
		return c.upsertFn(ctx, h.gvr, h.namespace, unstrob)
	}

	return err
}
