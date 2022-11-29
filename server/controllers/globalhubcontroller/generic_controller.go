package globalhubcontroller

import (
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type IGenericController interface {
	// Start starts N worker processes processing work items.
	Run(numThreads int)
}

type GenericController struct {
	name           string
	stopCh         <-chan struct{}
	gvr            schema.GroupVersionResource
	informer       cache.SharedIndexInformer
	queue          workqueue.RateLimitingInterface
	createInstance func() client.Object
	client         dynamic.Interface
	reconcile      func(stopCh <-chan struct{}, obj interface{}) error
}

func NewGenericController(stopChannel <-chan struct{}, client dynamic.Interface, informerFactory dynamicinformer.DynamicSharedInformerFactory, controller IController) *GenericController {
	c := &GenericController{
		stopCh:         stopChannel,
		name:           controller.GetName(),
		client:         client,
		gvr:            controller.GetGVR(),
		informer:       informerFactory.ForResource(controller.GetGVR()).Informer(),
		queue:          workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), controller.GetName()),
		createInstance: controller.CreateInstanceFunc(),
		reconcile:      controller.ReconcileFunc(),
	}

	c.informer.AddEventHandler(
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

// enqueue enqueues a resource.
func (c *GenericController) enqueue(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.queue.Add(key)
}

func (c *GenericController) Run(numThreads int) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	go c.informer.Run(c.stopCh)
	if !cache.WaitForCacheSync(c.stopCh, c.informer.HasSynced) {
		klog.Info("Timed out waiting for caches to sync")
		return
	}

	klog.Infof("Starting %s controller", c.name)
	defer klog.Infof("Shutting down %s controller", c.name)

	for i := 0; i < numThreads; i++ {
		go wait.Until(c.startWorker, time.Second, c.stopCh)
	}

	<-c.stopCh
}

func (c *GenericController) startWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *GenericController) processNextWorkItem() bool {
	// Wait until there is a new item in the working queue
	k, quit := c.queue.Get()
	if quit {
		return false
	}
	key := k.(string)
	// No matter what, tell the queue we're done with this key, to unblock
	// other workers.
	defer c.queue.Done(key)
	if err := c.process(key); err != nil {
		utilruntime.HandleError(fmt.Errorf("%q controller failed to sync %q, err: %w", c.name, key, err))
		c.queue.AddRateLimited(key)
		return true
	}
	c.queue.Forget(key)
	return true
}

func (c *GenericController) process(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		klog.Errorf("invalid key: %q: %v", key, err)
		return nil
	}

	klog.V(5).Infof("process object is: %s/%s", namespace, name)

	instance := c.createInstance()
	instance.SetName(name)
	instance.SetNamespace(namespace)

	item, exist, err := c.informer.GetStore().Get(instance)
	if !exist {
		klog.Warningf("cann't get object: %s/%s", namespace, name)
		return nil
	}
	if err != nil {
		klog.Errorf("get object(%s/%s) error %v", namespace, name, err)
		return nil
	}

	if err != c.reconcile(c.stopCh, item) {
		klog.Errorf("reconcile object(%s/%s) error %v", namespace, name, err)
		return err
	}
	return nil
}
