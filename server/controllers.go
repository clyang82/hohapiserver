package server

import (
	"context"
	"fmt"

	filteredcache "github.com/IBM/controller-filtered-cache/filteredcache"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
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

	scheme := runtime.NewScheme()

	if err := apiextensionsv1.AddToScheme(scheme); err != nil {
		return err
	}
	if err := policyv1.AddToScheme(scheme); err != nil {
		return err
	}
	if err := placementrulev1.AddToScheme(scheme); err != nil {
		return err
	}

	gvkLabelsMap := map[schema.GroupVersionKind][]filteredcache.Selector{
		apiextensionsv1.SchemeGroupVersion.WithKind("CustomResourceDefinition"): {
			{FieldSelector: fmt.Sprintf("metadata.name==%s", "policies.policy.open-cluster-management.io")},
			{FieldSelector: fmt.Sprintf("metadata.name==%s", "placementbindings.policy.open-cluster-management.io")},
			{FieldSelector: fmt.Sprintf("metadata.name==%s", "placementrules.apps.open-cluster-management.io")},
			{FieldSelector: fmt.Sprintf("metadata.name==%s", "managedclusters.cluster.open-cluster-management.io")},
			{FieldSelector: fmt.Sprintf("metadata.name==%s", "subscriptionreports.apps.open-cluster-management.io")},
			{FieldSelector: fmt.Sprintf("metadata.name==%s", "subscriptions.apps.open-cluster-management.io")},
			{FieldSelector: fmt.Sprintf("metadata.name==%s", "subscriptionstatuses.apps.open-cluster-management.io")},
		},
		policyv1.SchemeGroupVersion.WithKind("Policy"): {
			{LabelSelector: fmt.Sprint("!" + localResourceLabel)},
			{LabelSelector: fmt.Sprint("!" + rootPolicyLabel)},
		},
		policyv1.SchemeGroupVersion.WithKind("PolicyBinding"): {
			{LabelSelector: fmt.Sprint("!" + rootPolicyLabel)},
		},
		placementrulev1.SchemeGroupVersion.WithKind("PlacementRule"): {
			{LabelSelector: fmt.Sprint("!" + localResourceLabel)},
		},
	}

	opts := cache.Options{
		Scheme: scheme,
	}

	var err error
	s.Cache, err = filteredcache.NewEnhancedFilteredCacheBuilder(gvkLabelsMap)(s.hostedConfig, opts)
	if err != nil {
		return err
	}
	go s.Cache.Start(ctx)
	s.Cache.WaitForCacheSync(ctx)
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

	informer, err := s.Cache.GetInformerForKind(ctx, crdGVK)
	if err != nil {
		return err
	}
	// configure the dynamic informer event handlers
	c := controllers.NewGenericController(ctx, controllerName, dynamicClient, crdGVR, informer)

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

	informer, err := s.Cache.GetInformerForKind(ctx, gvk)
	if err != nil {
		return err
	}
	c := controllers.NewGenericController(ctx, "policy", dynamicClient, gvr, informer)

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
		Group:   placementrulev1.SchemeGroupVersion.Group,
		Version: placementrulev1.SchemeGroupVersion.Version,
		Kind:    "PlacementRule",
	}

	informer, err := s.Cache.GetInformerForKind(ctx, gvk)
	if err != nil {
		return err
	}
	c := controllers.NewGenericController(ctx, "placementrule", dynamicClient, gvr, informer)

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

	informer, err := s.Cache.GetInformerForKind(ctx, gvk)
	if err != nil {
		return err
	}
	c := controllers.NewGenericController(ctx, "placementbinding", dynamicClient, gvr, informer)

	s.AddPostStartHook("hoh-start-placementbinding-controller", func(hookContext genericapiserver.PostStartHookContext) error {
		go c.Run(ctx, 2)
		return nil
	})
	return nil
}
