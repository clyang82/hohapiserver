package controllers

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog"
	policyv1 "open-cluster-management.io/governance-policy-propagator/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	policysummaryv1alpha1 "github.com/clyang82/multicluster-global-hub-lite/apis/policysummary/v1alpha1"
)

type policyController struct {
	Controller
	name             string
	policyGVR        schema.GroupVersionResource
	client           dynamic.Interface
	policySummaryGVR schema.GroupVersionResource
	createInstance   func() client.Object
}

func NewPolicyController(dynamicClient dynamic.Interface) *policyController {
	return &policyController{
		name:             "policy-controller",
		policyGVR:        policyv1.SchemeGroupVersion.WithResource("policies"),
		policySummaryGVR: policysummaryv1alpha1.SchemeBuilder.GroupVersion.WithResource("policysummaries"),
		client:           dynamicClient,
		createInstance: func() client.Object {
			return &policyv1.Policy{}
		},
	}
}

func (c *policyController) GetName() string {
	return c.name
}

func (c *policyController) GetGVR() schema.GroupVersionResource {
	return c.policyGVR
}

func (c *policyController) CreateInstanceFunc() func() client.Object {
	return c.createInstance
}

func (c *policyController) ReconcileFunc() func(ctx context.Context, obj interface{}) error {
	return func(ctx context.Context, obj interface{}) error {
		unObj, ok := obj.(*unstructured.Unstructured)
		if !ok {
			return fmt.Errorf("cann't convert obj(%+v) to *unstructured.Unstructured", obj)
		}

		policy := &policyv1.Policy{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unObj.UnstructuredContent(), policy); err != nil {
			return err
		}

		policyStatus := policy.Status.Status
		if policyStatus == nil {
			klog.Infof("policy(%s/%s) status is empty", policy.GetNamespace(), policy.GetName())
			return nil
		}

		if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			return c.updatePolicySummary(ctx, policy)
		}); err != nil {
			return err
		}
		return nil
	}
}

func (c *policyController) updatePolicySummary(ctx context.Context, policy *policyv1.Policy) error {
	policySummary := &policysummaryv1alpha1.PolicySummary{}
	unStructObj, err := c.client.Resource(c.policySummaryGVR).Get(ctx, policy.GetName(), metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return err
	}
	if errors.IsNotFound(err) {
		policySummary = &policysummaryv1alpha1.PolicySummary{
			TypeMeta: metav1.TypeMeta{
				Kind:       "PolicySummary",
				APIVersion: "cluster.open-cluster-management.io/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: policy.GetName(),
			},
		}
		unStructMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(policySummary)
		if err != nil {
			return err
		}
		unStructObj, err = c.client.Resource(c.policySummaryGVR).Create(ctx,
			&unstructured.Unstructured{Object: unStructMap}, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	}

	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unStructObj.Object, policySummary); err != nil {
		return err
	}

	newRegionalHub := policysummaryv1alpha1.RegionalHubPolicyStatus{
		Name:         policy.GetNamespace(), // TODO: the policy namespace is the identity of global hub syncer
		Compliant:    0,
		NonCompliant: 0,
	}
	for _, cluster := range policy.Status.Status {
		switch cluster.ComplianceState {
		case policyv1.Compliant:
			newRegionalHub.Compliant++
		case policyv1.NonCompliant:
			newRegionalHub.NonCompliant++
		default:
			klog.Warningf("cluster %s with unknown status %s", cluster.ClusterName, cluster.ComplianceState)
		}
	}

	exist := false
	for index, regionalHub := range policySummary.Status.RegionalHubs {
		if newRegionalHub.Name == regionalHub.Name {
			exist = true
			policySummary.Status.Compliant += (newRegionalHub.Compliant - regionalHub.Compliant)
			policySummary.Status.NonCompliant += (newRegionalHub.NonCompliant - regionalHub.NonCompliant)
			policySummary.Status.RegionalHubs[index].Compliant = newRegionalHub.Compliant
			policySummary.Status.RegionalHubs[index].NonCompliant = newRegionalHub.NonCompliant
		}
	}
	if !exist {
		policySummary.Status.Compliant += newRegionalHub.Compliant
		policySummary.Status.NonCompliant += newRegionalHub.NonCompliant
		policySummary.Status.RegionalHubs = append(policySummary.Status.RegionalHubs, newRegionalHub)
	}

	unStructMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(policySummary)
	if err != nil {
		return err
	}
	if _, err := c.client.Resource(c.policySummaryGVR).UpdateStatus(context.TODO(),
		&unstructured.Unstructured{Object: unStructMap}, metav1.UpdateOptions{}); err != nil {
		return err
	}

	return nil
}
