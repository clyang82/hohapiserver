package server

import (
	"context"
	"fmt"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"
	policyv1 "open-cluster-management.io/governance-policy-propagator/api/v1"

	"github.com/clyang82/hohapiserver/controllers"
)

func (s *HoHApiServer) InstallPolicyController(ctx context.Context, config *rest.Config) error {
	config = rest.AddUserAgent(rest.CopyConfig(config), "hoh-policy-controller")
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		klog.Fatal(err)
	}

	gvr := schema.GroupVersionResource{
		Group:    policyv1.GroupVersion.Group,
		Version:  policyv1.GroupVersion.Version,
		Resource: "policies",
	}

	dynInformerFactory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(s.client, 0, "", func(o *metav1.ListOptions) {
		o.LabelSelector = fmt.Sprintf("!%s", "policy.open-cluster-management.io/root-policy")
	})
	c := controllers.NewGenericController(ctx, "policy-controller", dynamicClient, gvr)
	dynInformerFactory.ForResource(gvr).Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				c.Enqueue(obj)
			},
			UpdateFunc: func(_, obj interface{}) {
				c.Enqueue(obj)
			},
			DeleteFunc: func(obj interface{}) {
				c.Enqueue(obj)
			},
		},
	)
	dynInformerFactory.Start(ctx.Done())
	c.Indexer = dynInformerFactory.ForResource(gvr).Informer().GetIndexer()

	s.AddPostStartHook("hoh-start-policy-controller", func(hookContext genericapiserver.PostStartHookContext) error {
		go c.Run(ctx, 2)
		return nil
	})
	return nil
}

func (s *HoHApiServer) InstallCRDController(ctx context.Context, config *rest.Config) error {

	config = rest.AddUserAgent(rest.CopyConfig(config), "hoh-crd-controller")
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		klog.Fatal(err)
	}

	gvr := schema.GroupVersionResource{
		Group:    apiextensionsv1.SchemeGroupVersion.Group,
		Version:  apiextensionsv1.SchemeGroupVersion.Version,
		Resource: "customresourcedefinitions",
	}
	// configure the dynamic informer event handlers
	dynInformerFactory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(s.client, 0, "", func(o *metav1.ListOptions) {
		o.FieldSelector = fmt.Sprintf("metadata.name==%s", "policies.policy.open-cluster-management.io")
	})
	c := controllers.NewGenericController(ctx, "policy-controller", dynamicClient, gvr)
	dynInformerFactory.ForResource(gvr).Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				c.Enqueue(obj)
			},
			UpdateFunc: func(_, obj interface{}) {
				c.Enqueue(obj)
			},
			DeleteFunc: func(obj interface{}) {
				c.Enqueue(obj)
			},
		},
	)
	dynInformerFactory.Start(ctx.Done())
	c.Indexer = dynInformerFactory.ForResource(gvr).Informer().GetIndexer()

	s.AddPostStartHook("hoh-start-crd-controller", func(hookContext genericapiserver.PostStartHookContext) error {
		go c.Run(ctx, 2)
		return nil
	})
	return nil
}
