package policy

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	util "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"
)

type PolicyController struct {
	client          dynamic.Interface
	informerFactory dynamicinformer.DynamicSharedInformerFactory

	queue workqueue.RateLimitingInterface
	gvr   schema.GroupVersionResource
}

const (
	controllerName = "policy-controller"
)

func NewPolicyController(ctx context.Context, client dynamic.Interface,
	informerFactory dynamicinformer.DynamicSharedInformerFactory, gvr schema.GroupVersionResource) *PolicyController {

	queue := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), controllerName)

	c := &PolicyController{
		client:          client,
		informerFactory: informerFactory,
		queue:           queue,
		gvr:             gvr,
	}

	// configure the dynamic informer event handlers
	informerFactory.ForResource(c.gvr).Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				c.enqueue(obj)
			},
			UpdateFunc: func(_, obj interface{}) {
				c.enqueue(obj)
			},
			DeleteFunc: func(obj interface{}) {
				c.enqueue(obj)
			},
		},
	)
	return c
}

// enqueue enqueues an policy.
func (c *PolicyController) enqueue(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		util.HandleError(err)
		return
	}

	klog.V(2).Infof("Queueing policy %q", key)
	c.queue.Add(key)
}

func (c *PolicyController) Run(ctx context.Context, numThreads int) {
	defer util.HandleCrash()
	defer c.queue.ShutDown()

	klog.Infof("Starting %s controller", controllerName)
	defer klog.Infof("Shutting down %s controller", controllerName)

	for i := 0; i < numThreads; i++ {
		go wait.UntilWithContext(ctx, c.startWorker, time.Second)
	}

	<-ctx.Done()
}

func (c *PolicyController) startWorker(ctx context.Context) {
	for c.processNextWorkItem(ctx) {
	}
}

func (c *PolicyController) processNextWorkItem(ctx context.Context) bool {
	// Wait until there is a new item in the working queue
	k, quit := c.queue.Get()
	if quit {
		return false
	}
	key := k.(string)

	// No matter what, tell the queue we're done with this key, to unblock
	// other workers.
	defer c.queue.Done(key)

	if err := c.process(ctx, key); err != nil {
		util.HandleError(fmt.Errorf("%q controller failed to sync %q, err: %w", controllerName, key, err))
		c.queue.AddRateLimited(key)
		return true
	}
	c.queue.Forget(key)
	return true
}

func (c *PolicyController) process(ctx context.Context, key string) error {

	obj, err := c.informerFactory.ForResource(c.gvr).Lister().Get(key)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil // object deleted before we handled it
		}
		return err
	}
	// old := obj.(*policyv1.Policy)

	err = c.reconcile(ctx, obj)
	if err != nil {
		return err
	}

	// Regardless of whether reconcile returned an error or not, always try to patch status if needed. Return the
	// reconciliation error at the end.

	// If the object being reconciled changed as a result, update it.
	// if !equality.Semantic.DeepEqual(old.Status, obj.Status) {

	// }

	return nil
}

func (c *PolicyController) reconcile(ctx context.Context, obj runtime.Object) error {
	klog.Info("Starting to reconcile the policy")

	unstructuredObj := obj.(*unstructured.Unstructured)
	_, err := c.client.Resource(c.gvr).Create(ctx, unstructuredObj, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	return nil
}
