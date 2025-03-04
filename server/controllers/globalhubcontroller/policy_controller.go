package globalhubcontroller

import (
	"context"
	"encoding/json"
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

	policyv1 "open-cluster-management.io/governance-policy-propagator/api/v1"
)

const GlobalHubPolicyNamespaceLabel = "global-hub.open-cluster-management.io/original-namespace"

// ComplianceSummary ComplianceSummary `json:"complianceSummary,omitempty"` // used by global policy
type ComplianceSummary struct {
	Compliant    uint32           `json:"compliant,omitempty"`
	NonCompliant uint32           `json:"noncompliant,omitempty"`
	Summaries    []ClusterSummary `json:"summaries,omitempty"`
}

type ClusterSummary struct {
	Name         string `json:"name,omitempty"`
	Compliant    uint32 `json:"compliant,omitempty"`
	NonCompliant uint32 `json:"noncompliant,omitempty"`
}

type policyController struct {
	client    dynamic.Interface
	policyGVR schema.GroupVersionResource
}

func NewPolicyController(dynamicClient dynamic.Interface) IController {
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

func (c *policyController) ReconcileFunc() func(stopCh <-chan struct{}, obj interface{}) error {
	return func(stopCh <-chan struct{}, obj interface{}) error {
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
			if _, err := c.client.Resource(c.policyGVR).Namespace(unObj.GetNamespace()).Update(context.TODO(), unObj, metav1.UpdateOptions{}); err != nil {
				return err
			}
			klog.Infof("add global label to resource(%s/%s) \n", unObj.GetNamespace(), unObj.GetName())
			return nil
		}

		if originalNamespace == unObj.GetNamespace() {
			klog.Infof("the policy(%s/%s) is from global hub namespace, skip reconcile status", unObj.GetNamespace(), unObj.GetName())
			return nil
		}

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
			return c.updateGlobalHubPolicy(stopCh, syncerPolicy, originalNamespace)
		}); err != nil {
			return err
		}
		return nil
	}
}

func (c *policyController) updateGlobalHubPolicy(stopCh <-chan struct{}, syncerPolicy *policyv1.Policy, originalNamespace string) error {
	globalObj, err := c.client.Resource(c.policyGVR).Namespace(originalNamespace).Get(context.TODO(), syncerPolicy.GetName(), metav1.GetOptions{})
	if errors.IsNotFound(err) {
		klog.Errorf("the policy(%s) is not existed in global hub namespace(%s)", syncerPolicy.GetName(), originalNamespace)
		return err
	}
	if err != nil {
		return err
	}
	newClusterSummary := ClusterSummary{
		Name:         syncerPolicy.GetNamespace(), // syncer identity
		Compliant:    0,
		NonCompliant: 0,
	}

	for _, cluster := range syncerPolicy.Status.Status {
		switch cluster.ComplianceState {
		case policyv1.Compliant:
			newClusterSummary.Compliant++
		case policyv1.NonCompliant:
			newClusterSummary.NonCompliant++
		default:
			klog.Warningf("cluster %s with unknown status %s", cluster.ClusterName, cluster.ComplianceState)
		}
	}

	policyStatusMap := make(map[string]interface{})
	if globalObj.Object["status"] != nil {
		policyStatusMap = globalObj.Object["status"].(map[string]interface{})
	} else {
		globalObj.Object["status"] = policyStatusMap
	}

	policyComplianceSummary := ComplianceSummary{}
	policyComplianceSummaryJson, err := json.Marshal(policyStatusMap["complianceSummary"])
	if err != nil {
		return err
	}
	if err := json.Unmarshal(policyComplianceSummaryJson, &policyComplianceSummary); err != nil {
		return err
	}

	exist := false
	for index, complianceSummary := range policyComplianceSummary.Summaries {
		if newClusterSummary.Name == complianceSummary.Name {
			exist = true
			policyComplianceSummary.Compliant += (newClusterSummary.Compliant - complianceSummary.Compliant)
			policyComplianceSummary.NonCompliant += (newClusterSummary.NonCompliant - complianceSummary.NonCompliant)
			policyComplianceSummary.Summaries[index].Compliant = newClusterSummary.Compliant
			policyComplianceSummary.Summaries[index].NonCompliant = newClusterSummary.NonCompliant
		}
	}
	if !exist {
		policyComplianceSummary.Compliant += newClusterSummary.Compliant
		policyComplianceSummary.NonCompliant += newClusterSummary.NonCompliant
		policyComplianceSummary.Summaries = append(policyComplianceSummary.Summaries, newClusterSummary)
	}

	// convert struct to map
	policyComplianceSummaryStr, err := json.Marshal(policyComplianceSummary)
	if err != nil {
		return err
	}
	policyComplianceSummaryMap := make(map[string]interface{})
	if err := json.Unmarshal(policyComplianceSummaryStr, &policyComplianceSummaryMap); err != nil {
		return err
	}

	policyStatusMap["complianceSummary"] = policyComplianceSummaryMap

	if _, err := c.client.Resource(c.policyGVR).Namespace(globalObj.GetNamespace()).UpdateStatus(context.TODO(),
		globalObj, metav1.UpdateOptions{}); err != nil {
		return err
	}

	klog.Infof("updated global policy: %s/%s by syncer policy: %s/%s", globalObj.GetNamespace(), globalObj.GetName(), syncerPolicy.GetNamespace(), syncerPolicy.GetName())
	return nil
}
