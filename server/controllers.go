package server

import (
	"context"
	"fmt"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	toolscache "k8s.io/client-go/tools/cache"
	"k8s.io/klog"
	policyv1 "open-cluster-management.io/governance-policy-propagator/api/v1"
	placementrulev1 "open-cluster-management.io/multicloud-operators-subscription/pkg/apis/apps/placementrule/v1"
	"sigs.k8s.io/controller-runtime/pkg/cache"

	"github.com/clyang82/hohapiserver/controllers"
)

const (
	rootPolicyLabel    = "policy.open-cluster-management.io/root-policy"
	localResourceLabel = "hub-of-hubs.open-cluster-management.io/local-resource"
)

func (s *HoHApiServer) CreateCache(ctx context.Context) error {

	localResourceLabelReq, err := labels.NewRequirement(localResourceLabel, selection.DoesNotExist, []string{""})
	if err != nil {
		return err
	}

	opts := cache.Options{
		SelectorsByObject: cache.SelectorsByObject{
			&apiextensionsv1.CustomResourceDefinition{}: {
				Field: fields.SelectorFromSet(
					fields.Set(map[string]string{
						"metadata.name": "policies.policy.open-cluster-management.io,placementbindings.policy.open-cluster-management.io,placementrules.apps.open-cluster-management.io,managedclusters.cluster.open-cluster-management.io,subscriptionreports.apps.open-cluster-management.io,subscriptions.apps.open-cluster-management.io,subscriptionstatuses.apps.open-cluster-management.io",
					}))},
			&policyv1.Policy{}:               {Label: labels.NewSelector().Add(*localResourceLabelReq)},
			&policyv1.PlacementBinding{}:     {Label: labels.NewSelector().Add(*localResourceLabelReq)},
			&placementrulev1.PlacementRule{}: {Label: labels.NewSelector().Add(*localResourceLabelReq)},
		},
	}

	s.Cache, err = cache.New(s.hostedConfig, opts)
	if err != nil {
		return err
	}
	return nil
}

func (s *HoHApiServer) InstallCRDController(ctx context.Context, config *rest.Config) error {

	controllerName := "hoh-crd-controller"
	config = rest.AddUserAgent(rest.CopyConfig(config), controllerName)
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		klog.Fatal(err)
	}

	crdGVK := schema.GroupVersionKind{
		Group:   apiextensionsv1.SchemeGroupVersion.Group,
		Version: apiextensionsv1.SchemeGroupVersion.Version,
		Kind:    "CustomResourceDefinition",
	}

	crdGVR := schema.GroupVersionResource{
		Group:    apiextensionsv1.SchemeGroupVersion.Group,
		Version:  apiextensionsv1.SchemeGroupVersion.Version,
		Resource: "customresourcedefinitions",
	}

	// configure the dynamic informer event handlers
	c := controllers.NewGenericController(ctx, controllerName, dynamicClient, crdGVR)
	informer, err := s.Cache.GetInformerForKind(ctx, crdGVK)
	if err != nil {
		return err
	}
	informer.AddEventHandler(
		toolscache.ResourceEventHandlerFuncs{
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

	s.AddPostStartHook(fmt.Sprintf("start-%s", controllerName), func(hookContext genericapiserver.PostStartHookContext) error {
		go c.Run(ctx, 2)
		return nil
	})
	return nil
}

func (s *HoHApiServer) InstallPolicyController(ctx context.Context, config *rest.Config) error {
	config = rest.AddUserAgent(rest.CopyConfig(config), "hoh-policy")
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		klog.Fatal(err)
	}

	gvr := schema.GroupVersionResource{
		Group:    policyv1.GroupVersion.Group,
		Version:  policyv1.GroupVersion.Version,
		Resource: "policies",
	}
	gvk := schema.GroupVersionKind{
		Group:   policyv1.GroupVersion.Group,
		Version: policyv1.GroupVersion.Version,
		Kind:    "Policy",
	}

	c := controllers.NewGenericController(ctx, "policy", dynamicClient, gvr)
	informer, err := s.Cache.GetInformerForKind(ctx, gvk)
	if err != nil {
		return err
	}
	informer.AddEventHandler(
		toolscache.ResourceEventHandlerFuncs{
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
	config = rest.AddUserAgent(rest.CopyConfig(config), "hoh-placementrule")
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		klog.Fatal(err)
	}

	gvr := schema.GroupVersionResource{
		Group:    placementrulev1.SchemeGroupVersion.Group,
		Version:  placementrulev1.SchemeGroupVersion.Version,
		Resource: "placementrules",
	}
	gvk := schema.GroupVersionKind{
		Group:   policyv1.GroupVersion.Group,
		Version: policyv1.GroupVersion.Version,
		Kind:    "PlacementRule",
	}

	c := controllers.NewGenericController(ctx, "placementrule", dynamicClient, gvr)
	informer, err := s.Cache.GetInformerForKind(ctx, gvk)
	if err != nil {
		return err
	}
	informer.AddEventHandler(
		toolscache.ResourceEventHandlerFuncs{
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
	config = rest.AddUserAgent(rest.CopyConfig(config), "hoh-placementbinding")
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		klog.Fatal(err)
	}

	gvr := schema.GroupVersionResource{
		Group:    policyv1.GroupVersion.Group,
		Version:  policyv1.GroupVersion.Version,
		Resource: "placementbindings",
	}
	gvk := schema.GroupVersionKind{
		Group:   policyv1.GroupVersion.Group,
		Version: policyv1.GroupVersion.Version,
		Kind:    "PlacementBinding",
	}

	c := controllers.NewGenericController(ctx, "placementbinding", dynamicClient, gvr)
	informer, err := s.Cache.GetInformerForKind(ctx, gvk)
	if err != nil {
		return err
	}
	informer.AddEventHandler(
		toolscache.ResourceEventHandlerFuncs{
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
