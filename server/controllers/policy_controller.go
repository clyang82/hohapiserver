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

	policyv1 "github.com/clyang82/multicluster-global-hub-lite/apis/policy/v1"
)

const GlobalHubPolicyNamespaceLabel = "global-hub.open-cluster-management.io/original-namespace"

type policyController struct {
	client    dynamic.Interface
	policyGVR schema.GroupVersionResource
}

func NewPolicyController(dynamicClient dynamic.Interface) Controller {
	return &policyController{
		client:    dynamicClient,
		policyGVR: policyv1.SchemeGroupVersion.WithResource("policies"),
	}
}

func (c *policyController) GetName() string {
	return "policy-controller"
}

func (c *policyController) GetGVR() schema.GroupVersionResource {
	return c.policyGVR
}

func (c *policyController) CreateInstanceFunc() func() client.Object {
	return func() client.Object {
		return &policyv1.Policy{}
	}
}

func (c *policyController) ReconcileFunc() func(ctx context.Context, obj interface{}) error {
	return func(ctx context.Context, obj interface{}) error {
		unObj, ok := obj.(*unstructured.Unstructured)
		if !ok {
			return fmt.Errorf("cann't convert obj(%+v) to *unstructured.Unstructured", obj)
		}
		// check the original namespace
		labels := unObj.GetLabels()
		originalNamespace, ok := labels[GlobalHubPolicyNamespaceLabel]

		if !ok {
			if labels == nil {
				labels = map[string]string{}
			}
			labels[GlobalHubPolicyNamespaceLabel] = unObj.GetNamespace()
			unObj.SetLabels(labels)
			if _, err := c.client.Resource(c.policyGVR).Namespace(unObj.GetNamespace()).Update(ctx, unObj, metav1.UpdateOptions{}); err != nil {
				return err
			}
			klog.Infof("add global label to resource(%s/%s) \n", unObj.GetNamespace(), unObj.GetName())
			return nil
		}

		// TODO: remove the following comments if the
		// if originalNamespace == unObj.GetNamespace() {
		// 	klog.Infof("the policy(%s/%s) is from global hub namespace, skip reconcile status", unObj.GetNamespace(), unObj.GetName())
		// 	return nil
		// }

		// watch the syncer's policy and update the global hub's policy
		syncerPolicy := &policyv1.Policy{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unObj.UnstructuredContent(), syncerPolicy); err != nil {
			return err
		}
		if syncerPolicy.Status.Status == nil {
			klog.Infof("policy(%s/%s) status is empty", syncerPolicy.GetNamespace(), syncerPolicy.GetName())
			return nil
		}

		// update the global hub policy status
		if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			return c.updateGlobalHubPolicy(ctx, syncerPolicy, originalNamespace)
		}); err != nil {
			return err
		}
		return nil
	}
}

func (c *policyController) updateGlobalHubPolicy(ctx context.Context, policy *policyv1.Policy, originalNamespace string) error {
	unStructObj, err := c.client.Resource(c.policyGVR).Namespace(originalNamespace).Get(ctx, policy.GetName(), metav1.GetOptions{})
	if errors.IsNotFound(err) {
		klog.Errorf("the policy(%s) is not existed in global hub namespace(%s)", policy.GetName(), originalNamespace)
		return err
	}
	if err != nil {
		return err
	}
	globalHubPolicy := &policyv1.Policy{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unStructObj.UnstructuredContent(), globalHubPolicy); err != nil {
		return err
	}

	newComplianceSummary := policyv1.ClusterSummary{
		Name:         policy.GetNamespace(),
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
	for index, complianceSummary := range globalHubPolicy.Status.ComplianceSummary.Summaries {
		if newComplianceSummary.Name == complianceSummary.Name {
			exist = true
			globalHubPolicy.Status.ComplianceSummary.Compliant += (newComplianceSummary.Compliant - complianceSummary.Compliant)
			globalHubPolicy.Status.ComplianceSummary.NonCompliant += (newComplianceSummary.NonCompliant - complianceSummary.NonCompliant)
			globalHubPolicy.Status.ComplianceSummary.Summaries[index].Compliant = newComplianceSummary.Compliant
			globalHubPolicy.Status.ComplianceSummary.Summaries[index].NonCompliant = newComplianceSummary.NonCompliant
		}
	}
	if !exist {
		globalHubPolicy.Status.ComplianceSummary.Compliant += newComplianceSummary.Compliant
		globalHubPolicy.Status.ComplianceSummary.NonCompliant += newComplianceSummary.NonCompliant
		globalHubPolicy.Status.ComplianceSummary.Summaries = append(policy.Status.ComplianceSummary.Summaries, newComplianceSummary)
	}

	unStructMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(globalHubPolicy)
	if err != nil {
		return err
	}

	if _, err := c.client.Resource(c.policyGVR).Namespace(globalHubPolicy.Namespace).UpdateStatus(context.TODO(),
		&unstructured.Unstructured{Object: unStructMap}, metav1.UpdateOptions{}); err != nil {
		return err
	}

	klog.Infof("updated global policy: %s/%s by syncer policy: %s/%s", globalHubPolicy.GetNamespace(), globalHubPolicy.GetName(), policy.GetNamespace(), policy.GetName())
	return nil
}
