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
	"sigs.k8s.io/controller-runtime/pkg/client"

	// policysummaryv1alpha1 "github.com/clyang82/multicluster-global-hub-lite/apis/policysummary/v1alpha1"
	policyv1 "github.com/clyang82/multicluster-global-hub-lite/apis/policy/v1"
)

const GlobalHubPolicyNamespace = "global-hub.open-cluster-management.io/original-namespace"

type policyController struct {
	Controller
	name           string
	policyGVR      schema.GroupVersionResource
	client         dynamic.Interface
	createInstance func() client.Object
}

func NewPolicyController(dynamicClient dynamic.Interface) *policyController {
	return &policyController{
		name:      "policy-controller",
		policyGVR: policyv1.SchemeGroupVersion.WithResource("policies"),
		client:    dynamicClient,
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
	unStructObj, err := c.client.Resource(c.policyGVR).Namespace(policy.GetNamespace()).Get(ctx, policy.GetName(), metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return err
	}
	if err != nil {
		unStructMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(policy)
		if err != nil {
			return err
		}
		unStructObj, err = c.client.Resource(c.policyGVR).Namespace(policy.Namespace).Create(ctx, &unstructured.Unstructured{Object: unStructMap}, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	}
	policy.SetResourceVersion(unStructObj.GetResourceVersion())

	newComplianceSummary := policyv1.ClusterSummary{
		Name:         policy.GetNamespace(), // TODO: the policy namespace is the identity of global hub syncer
		Compliant:    0,
		NonCompliant: 0,
	}
	for _, cluster := range policy.Status.Status {
		switch cluster.ComplianceState {
		case policyv1.Compliant:
			newComplianceSummary.Compliant++
		case policyv1.NonCompliant:
			newComplianceSummary.NonCompliant++
		default:
			klog.Warningf("cluster %s with unknown status %s", cluster.ClusterName, cluster.ComplianceState)
		}
	}

	exist := false
	for index, complianceSummary := range policy.Status.ComplianceSummary.Summaries {
		if newComplianceSummary.Name == complianceSummary.Name {
			exist = true
			policy.Status.ComplianceSummary.Compliant += (newComplianceSummary.Compliant - complianceSummary.Compliant)
			policy.Status.ComplianceSummary.NonCompliant += (newComplianceSummary.NonCompliant - complianceSummary.NonCompliant)
			policy.Status.ComplianceSummary.Summaries[index].Compliant = newComplianceSummary.Compliant
			policy.Status.ComplianceSummary.Summaries[index].NonCompliant = newComplianceSummary.NonCompliant
		}
	}
	if !exist {
		policy.Status.ComplianceSummary.Compliant += newComplianceSummary.Compliant
		policy.Status.ComplianceSummary.NonCompliant += newComplianceSummary.NonCompliant
		policy.Status.ComplianceSummary.Summaries = append(policy.Status.ComplianceSummary.Summaries, newComplianceSummary)
	}

	unStructMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(policy)
	if err != nil {
		return err
	}

	if _, err := c.client.Resource(c.policyGVR).Namespace(policy.Namespace).UpdateStatus(context.TODO(),
		&unstructured.Unstructured{Object: unStructMap}, metav1.UpdateOptions{}); err != nil {
		return err
	}

	return nil
}
