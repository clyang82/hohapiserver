package controllers

import (
	"context"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	toolscache "k8s.io/client-go/tools/cache"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/cache"
)

type Controller interface {
	Reconcile(ctx context.Context, obj interface{}) error
	Run(ctx context.Context)
}

type GenericController struct {
	context context.Context
	name    string
	// client is used to apply resources
	client dynamic.Interface
	// informer is used to watch the hosted resources
	informer cache.Informer

	gvr schema.GroupVersionResource

	Controller
}

func NewGenericController(ctx context.Context, name string, client dynamic.Interface,
	gvr schema.GroupVersionResource, informer cache.Informer) *GenericController {

	c := &GenericController{
		context:  ctx,
		name:     name,
		client:   client,
		gvr:      gvr,
		informer: informer,
	}

	c.informer.AddEventHandler(
		toolscache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				c.Reconcile(c.context, obj)
			},
			UpdateFunc: func(_, obj interface{}) {
				c.Reconcile(c.context, obj)
			},
			DeleteFunc: func(obj interface{}) {
				c.Reconcile(c.context, obj)
			},
		},
	)
	return c
}

func (c *GenericController) Run(ctx context.Context, numThreads int) {

	klog.Infof("Starting %s", c.name)

	<-ctx.Done()
}

func (c *GenericController) Reconcile(ctx context.Context, obj interface{}) error {
	klog.Info("Starting to reconcile the resource")

	tempObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return err
	}
	unstructuredObj := &unstructured.Unstructured{Object: tempObj}

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
