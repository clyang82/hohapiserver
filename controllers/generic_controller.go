package controllers

import (
	"context"
	"fmt"
	"time"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"
)

type GenericController struct {
	name string
	// client is used to apply resources
	client dynamic.Interface
	// informer is used to watch the hosted resources
	Informer informers.GenericInformer

	handler cache.ResourceEventHandler

	queue    workqueue.RateLimitingInterface
	Indexers cache.Indexers
	gvr      schema.GroupVersionResource
}

func NewGenericController(ctx context.Context, name string, client dynamic.Interface,
	gvr schema.GroupVersionResource) *GenericController {

	queue := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), name)

	c := &GenericController{
		name:   name,
		client: client,
		queue:  queue,
		gvr:    gvr,
	}

	return c
}

// enqueue enqueues a resource.
func (c *GenericController) Enqueue(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.queue.Add(key)
}

func (c *GenericController) Run(ctx context.Context, numThreads int) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	klog.Infof("Starting %s controller", c.name)
	defer klog.Infof("Shutting down %s controller", c.name)

	for i := 0; i < numThreads; i++ {
		go wait.UntilWithContext(ctx, c.startWorker, time.Second)
	}

	<-ctx.Done()
}

func (c *GenericController) startWorker(ctx context.Context) {
	for c.processNextWorkItem(ctx) {
	}
}

func (c *GenericController) processNextWorkItem(ctx context.Context) bool {
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
		utilruntime.HandleError(fmt.Errorf("%q controller failed to sync %q, err: %w", c.name, key, err))
		c.queue.AddRateLimited(key)
		return true
	}
	c.queue.Forget(key)
	return true
}

func (c *GenericController) process(ctx context.Context, key string) error {

	obj, _, err := c.Indexer.GetByKey(key)
	if err != nil {
		return err
	}

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

func (c *GenericController) reconcile(ctx context.Context, obj interface{}) error {
	klog.Info("Starting to reconcile the resource")
	unstructuredObj := obj.(*unstructured.Unstructured)

	//clean up unneeded fields
	manipulateObj(unstructuredObj)

	if unstructuredObj.GetNamespace() != "" {
		runtimeObj, err := c.client.Resource(c.gvr).Namespace(unstructuredObj.GetNamespace()).
			Get(ctx, unstructuredObj.GetName(), metav1.GetOptions{})
		if err != nil {
			if k8serrors.IsNotFound(err) {
				_, err = c.client.Resource(c.gvr).Namespace(unstructuredObj.GetNamespace()).
					Create(ctx, unstructuredObj, metav1.CreateOptions{})
				if err != nil {
					klog.Errorf("failed to create %s: %v", unstructuredObj.GetKind(), err)
					return err
				}
				return nil
			}
			klog.Errorf("failed to list %s: %v", unstructuredObj.GetKind(), err)
			return err
		}
		unstructuredObj.SetResourceVersion(runtimeObj.GetResourceVersion())
		_, err = c.client.Resource(c.gvr).Namespace(unstructuredObj.GetNamespace()).
			Update(ctx, unstructuredObj, metav1.UpdateOptions{})
		if err != nil {
			klog.Errorf("failed to update %s: %v", unstructuredObj.GetKind(), err)
			return err
		}
	} else {
		runtimeObj, err := c.client.Resource(c.gvr).
			Get(ctx, unstructuredObj.GetName(), metav1.GetOptions{})
		if err != nil {
			if k8serrors.IsNotFound(err) {
				_, err = c.client.Resource(c.gvr).
					Create(ctx, unstructuredObj, metav1.CreateOptions{})
				if err != nil {
					klog.Errorf("failed to create %s: %v", unstructuredObj.GetKind(), err)
					return err
				}
				return nil
			}
			klog.Errorf("failed to list %s: %v", unstructuredObj.GetKind(), err)
			return err
		}

		unstructuredObj.SetResourceVersion(runtimeObj.GetResourceVersion())
		_, err = c.client.Resource(c.gvr).
			Update(ctx, unstructuredObj, metav1.UpdateOptions{})
		if err != nil {
			klog.Errorf("failed to update %s: %v", unstructuredObj.GetKind(), err)
			return err
		}
	}

	return nil
}

func manipulateObj(unstructuredObj *unstructured.Unstructured) {
	unstructuredObj.SetUID("")
	unstructuredObj.SetResourceVersion("")
	unstructuredObj.SetManagedFields(nil)
	unstructuredObj.SetFinalizers(nil)
	unstructuredObj.SetGeneration(0)
	unstructuredObj.SetOwnerReferences(nil)
	unstructuredObj.SetClusterName("")

	delete(unstructuredObj.GetAnnotations(), "kubectl.kubernetes.io/last-applied-configuration")

}
