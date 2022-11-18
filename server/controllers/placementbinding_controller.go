package controllers

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	policyv1 "open-cluster-management.io/governance-policy-propagator/api/v1"
	placementrulev1 "open-cluster-management.io/multicloud-operators-subscription/pkg/apis/apps/placementrule/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type placementBindingController struct {
	client dynamic.Interface
	gvr    schema.GroupVersionResource
}

func NewPlacementBindingController(dynamicClient dynamic.Interface) Controller {
	return &placementBindingController{
		client: dynamicClient,
		gvr:    placementrulev1.SchemeGroupVersion.WithResource("placementbindings"),
	}
}

func (c *placementBindingController) GetName() string {
	return "placementbinding-controller"
}

func (c *placementBindingController) GetGVR() schema.GroupVersionResource {
	return c.gvr
}

func (c *placementBindingController) CreateInstanceFunc() func() client.Object {
	return func() client.Object {
		return &policyv1.PlacementBinding{}
	}
}

func (c *placementBindingController) ReconcileFunc() func(ctx context.Context, obj interface{}) error {
	return func(ctx context.Context, obj interface{}) error {
		unObj, ok := obj.(*unstructured.Unstructured)
		if !ok {
			return fmt.Errorf("cann't convert obj(%+v) to *unstructured.Unstructured", obj)
		}

		labels := unObj.GetLabels()
		_, ok = labels[GlobalHubPolicyNamespaceLabel]
		if !ok {
			labels[GlobalHubPolicyNamespaceLabel] = unObj.GetNamespace()
			if _, err := c.client.Resource(c.gvr).Namespace(unObj.GetNamespace()).Update(ctx, unObj, metav1.UpdateOptions{}); err != nil {
				return err
			}
			return nil
		}
		return nil
	}
}
