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
	placementrulev1 "open-cluster-management.io/multicloud-operators-subscription/pkg/apis/apps/placementrule/v1"

	"github.com/clyang82/hohapiserver/controllers"
)

const (
	rootPolicyLabel    = "policy.open-cluster-management.io/root-policy"
	localResourceLabel = "hub-of-hubs.open-cluster-management.io/local-resource"
)

func (s *HoHApiServer) InstallCRDController(ctx context.Context, config *rest.Config) error {

	config = rest.AddUserAgent(rest.CopyConfig(config), "hoh-crd-controller")
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		klog.Fatal(err)
	}

	crds := []string{
		"policies.policy.open-cluster-management.io",
		"placementbindings.policy.open-cluster-management.io",
		"placementrules.apps.open-cluster-management.io",
		"managedclusters.cluster.open-cluster-management.io",
		"subscriptionreports.apps.open-cluster-management.io",
		"subscriptions.apps.open-cluster-management.io",
		"subscriptionstatuses.apps.open-cluster-management.io",
	}

	crdGVR := schema.GroupVersionResource{
		Group:    apiextensionsv1.SchemeGroupVersion.Group,
		Version:  apiextensionsv1.SchemeGroupVersion.Version,
		Resource: "customresourcedefinitions",
	}
	for _, name := range crds {

		// configure the dynamic informer event handlers
		c := controllers.NewGenericController(ctx, fmt.Sprintf("%s-controller", name), dynamicClient, crdGVR)
		c.Indexers = s.dynInformerFactory.ForResource(crdGVR).Informer().GetIndexer().GetIndexers()

		c.Informer = dynamicinformer.NewFilteredDynamicInformer(s.client, crdGVR, "", 0, c.Indexers, func(o *metav1.ListOptions) {
			o.FieldSelector = fmt.Sprintf("%s=%s", "metadata.name", name)
		})
		c.Informer.Informer().AddEventHandler(
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

		s.AddPostStartHook("hoh-start-crd-controller", func(hookContext genericapiserver.PostStartHookContext) error {
			go c.Run(ctx, 2)
			return nil
		})

	}
	return nil
}

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

	c := controllers.NewGenericController(ctx, "policy-controller", dynamicClient, gvr)
	c.Indexers = s.dynInformerFactory.ForResource(gvr).Informer().GetIndexer().GetIndexers()
	c.Informer = dynamicinformer.NewFilteredDynamicInformer(s.client, gvr, "", 0, c.Indexers, func(o *metav1.ListOptions) {
		o.LabelSelector = fmt.Sprintf("!%s,!%s", rootPolicyLabel, localResourceLabel)
	})
	c.Informer.Informer().AddEventHandler(
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

	s.AddPostStartHook("hoh-start-policy-controller", func(hookContext genericapiserver.PostStartHookContext) error {
		go c.Run(ctx, 2)
		return nil
	})
	return nil
}

func (s *HoHApiServer) InstallPlacementRuleController(ctx context.Context, config *rest.Config) error {
	config = rest.AddUserAgent(rest.CopyConfig(config), "hoh-placementrule-controller")
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		klog.Fatal(err)
	}

	gvr := schema.GroupVersionResource{
		Group:    placementrulev1.SchemeGroupVersion.Group,
		Version:  placementrulev1.SchemeGroupVersion.Version,
		Resource: "placementrules",
	}

	c := controllers.NewGenericController(ctx, "policy-controller", dynamicClient, gvr)
	c.Indexers = s.dynInformerFactory.ForResource(gvr).Informer().GetIndexer().GetIndexers()
	c.Informer = dynamicinformer.NewFilteredDynamicInformer(s.client, gvr, "", 0, c.Indexers, func(o *metav1.ListOptions) {
		o.LabelSelector = fmt.Sprintf("!%s", localResourceLabel)
	})
	c.Informer.Informer().AddEventHandler(
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

	s.AddPostStartHook("hoh-start-placementrule-controller", func(hookContext genericapiserver.PostStartHookContext) error {
		go c.Run(ctx, 2)
		return nil
	})
	return nil
}

func (s *HoHApiServer) InstallPlacementBindingController(ctx context.Context, config *rest.Config) error {
	config = rest.AddUserAgent(rest.CopyConfig(config), "hoh-placementbinding-controller")
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		klog.Fatal(err)
	}

	gvr := schema.GroupVersionResource{
		Group:    policyv1.GroupVersion.Group,
		Version:  policyv1.GroupVersion.Version,
		Resource: "placementbindings",
	}

	c := controllers.NewGenericController(ctx, "policy-controller", dynamicClient, gvr)
	c.Indexers = s.dynInformerFactory.ForResource(gvr).Informer().GetIndexer().GetIndexers()
	c.Informer = dynamicinformer.NewFilteredDynamicInformer(s.client, gvr, "", 0, c.Indexers, func(o *metav1.ListOptions) {
		o.LabelSelector = fmt.Sprintf("!%s", localResourceLabel)
	})
	c.Informer.Informer().AddEventHandler(
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

	s.AddPostStartHook("hoh-start-placementbinding-controller", func(hookContext genericapiserver.PostStartHookContext) error {
		go c.Run(ctx, 2)
		return nil
	})
	return nil
}
