package controllers

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/controller-runtime/pkg/client"

	placementrulev1 "open-cluster-management.io/multicloud-operators-subscription/pkg/apis/apps/placementrule/v1"
)

type placementRuleController struct {
	client dynamic.Interface
	gvr    schema.GroupVersionResource
}

func NewPlacementRuleController(dynamicClient dynamic.Interface) Controller {
	return &placementRuleController{
		client: dynamicClient,
		gvr:    placementrulev1.SchemeGroupVersion.WithResource("placementrules"),
	}
}

func (c *placementRuleController) GetName() string {
	return "placementrule-controller"
}

func (c *placementRuleController) GetGVR() schema.GroupVersionResource {
	return c.gvr
}

func (c *placementRuleController) CreateInstanceFunc() func() client.Object {
	return func() client.Object {
		return &placementrulev1.PlacementRule{}
	}
}

func (c *placementRuleController) ReconcileFunc() func(ctx context.Context, obj interface{}) error {
	return func(ctx context.Context, obj interface{}) error {
		unObj, ok := obj.(*unstructured.Unstructured)
		if !ok {
			return fmt.Errorf("cann't convert obj(%+v) to *unstructured.Unstructured", obj)
		}

		labels := unObj.GetLabels()
		_, ok = labels[GlobalHubPolicyNamespaceLabel]
		if !ok {
			if labels == nil {
				labels = map[string]string{}
			}
			labels[GlobalHubPolicyNamespaceLabel] = unObj.GetNamespace()
			unObj.SetLabels(labels)
			if _, err := c.client.Resource(c.gvr).Namespace(unObj.GetNamespace()).Update(ctx, unObj, metav1.UpdateOptions{}); err != nil {
				return err
			}
			return nil
		}
		return nil
	}
}
