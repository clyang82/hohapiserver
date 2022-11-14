package controllers

import (
	"context"
	"encoding/json"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog"
	policyv1 "open-cluster-management.io/governance-policy-propagator/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	policysummaryv1alpha1 "github.com/clyang82/multicluster-global-hub-lite/apis/policysummary/v1alpha1"
)

type policyController struct {
	Controller
	name           string
	gvr            *schema.GroupVersionResource
	client         dynamic.Interface
	createInstance func() client.Object
}

func NewPolicyController(dynamicClient dynamic.Interface) *policyController {
	gvr, _ := schema.ParseResourceArg("policies.v1.policy.open-cluster-management.io")
	return &policyController{
		name:   "policy-controller",
		gvr:    gvr,
		client: dynamicClient,
		createInstance: func() client.Object {
			return &policyv1.Policy{}
		},
	}
}

func (c *policyController) GetName() string {
	return c.name
}

func (c *policyController) GetGVR() schema.GroupVersionResource {
	return *c.gvr
}

func (c *policyController) CreateInstanceFunc() func() client.Object {
	return c.createInstance
}

func (c *policyController) ReconcileFunc() func(ctx context.Context, obj interface{}) error {
	return func(ctx context.Context, obj interface{}) error {
		unObj, ok := obj.(*unstructured.Unstructured)
		if !ok {
			klog.Errorf("cann't convert obj(%+v) to *unstructured.Unstructured", obj)
			return nil
		}

		policy := &policyv1.Policy{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unObj.UnstructuredContent(), policy); err != nil {
			return err
		}

		klog.Info("the policy: ")
		prettyPrint(policy)

		policyStatus := policy.Status.Status
		if policyStatus == nil {
			klog.Infof("policy(%s/%s) with empty status", policy.GetNamespace(), policy.GetName())
			return nil
		}

		newRegionalHub := policysummaryv1alpha1.RegionalHubPolicyStatus{
			Name:         policy.GetNamespace(),
			Compliant:    0,
			NonCompliant: 0,
		}
		for _, cluster := range policyStatus {
			switch cluster.ComplianceState {
			case policyv1.Compliant:
				newRegionalHub.Compliant++
			case policyv1.NonCompliant:
				newRegionalHub.NonCompliant++
			default:
				klog.Warningf("cluster %s with unknown status %s", cluster.ClusterName, cluster.ComplianceState)
			}
		}
		klog.Info("the newRegionalHub: ")
		prettyPrint(newRegionalHub)

		policySummary, err := c.getPolicySummary(ctx, policy)
		if err != nil {
			return err
		}

		klog.Info("init policySummary: ")
		prettyPrint(newRegionalHub)

		exist := false
		for _, regionalHub := range policySummary.Status.RegionalHubs {
			if newRegionalHub.Name == regionalHub.Name {
				exist = true
				deltaCompliant := newRegionalHub.Compliant - regionalHub.Compliant
				deltaNonCompliant := newRegionalHub.NonCompliant - regionalHub.NonCompliant
				policySummary.Status.Compliant += deltaCompliant
				policySummary.Status.NonCompliant += deltaNonCompliant
				regionalHub.Compliant = newRegionalHub.Compliant
				regionalHub.NonCompliant = newRegionalHub.NonCompliant
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
		unStructObj := &unstructured.Unstructured{Object: unStructMap}
		updateObj, err := c.client.Resource(policysummaryv1alpha1.SchemeBuilder.GroupVersion.WithResource("policysummaries")).Update(ctx, unStructObj, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
		klog.Info("update policySummary: ")
		prettyPrint(updateObj)
		return nil
	}
}

func (c *policyController) getPolicySummary(ctx context.Context, policy *policyv1.Policy) (*policysummaryv1alpha1.PolicySummary, error) {
	policySummary := &policysummaryv1alpha1.PolicySummary{}
	unStructObj, err := c.client.Resource(policysummaryv1alpha1.SchemeBuilder.GroupVersion.WithResource("policysummaries")).Get(ctx, policy.GetName(), metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	}
	if err == nil {
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unStructObj.Object, policySummary); err != nil {
			return nil, err
		}
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
			Status: policysummaryv1alpha1.PolicySummaryStatus{
				Compliant:    0,
				NonCompliant: 0,
				RegionalHubs: []policysummaryv1alpha1.RegionalHubPolicyStatus{
					{
						Name:         policy.GetNamespace(),
						Compliant:    0,
						NonCompliant: 0,
					},
				},
			},
		}
		unStructMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(policySummary)
		if err != nil {
			return nil, err
		}
		unStructObj := &unstructured.Unstructured{Object: unStructMap}
		if _, err := c.client.Resource(policysummaryv1alpha1.SchemeBuilder.GroupVersion.WithResource("policysummaries")).Create(ctx, unStructObj, metav1.CreateOptions{}); err != nil {
			return nil, err
		}
	}
	return policySummary, nil
}

func prettyPrint(obj any) {
	bytes, _ := json.MarshalIndent(obj, "", "  ")
	klog.Info(string(bytes))
}
