package controllers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	policyv1 "github.com/clyang82/multicluster-global-hub-lite/apis/policy/v1"
	"github.com/clyang82/multicluster-global-hub-lite/server/controllers"
)

var (
	cfg       *rest.Config
	client    dynamic.Interface
	testEnv   *envtest.Environment
	policyGVR schema.GroupVersionResource
)

func TestMain(m *testing.M) {
	// start testEnv
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "manifests"),
		},
	}
	policyGVR = policyv1.SchemeGroupVersion.WithResource("policies")

	var err error
	cfg, err = testEnv.Start()
	if err != nil {
		panic(err)
	}

	client, err = dynamic.NewForConfig(cfg)
	if err != nil {
		panic(err)
	}

	// run testings
	code := m.Run()

	// stop testEnv
	err = testEnv.Stop()
	if err != nil {
		panic(err)
	}

	os.Exit(code)
}

func TestPolicySummary(t *testing.T) {
	// 1. get the reconcile function
	policyController := controllers.NewPolicyController(client)
	reconcileFunc := policyController.ReconcileFunc()

	// 2. create namespace for global hub and syncer
	if err := createNamespace(context.TODO(), "global-hub"); err != nil {
		t.Fatal(fmt.Errorf("error to namespace global-hub: %w", err))
	}
	if err := createNamespace(context.TODO(), "syncer"); err != nil {
		t.Fatal(fmt.Errorf("error to namespace syncer: %w", err))
	}

	// 2. create policy test-policy in global-hub policy
	policy1 := &policyv1.Policy{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Policy",
			APIVersion: "policy.open-cluster-management.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-policy",
			Namespace: "default", // global-hub
			// Labels: map[string]string{
			// 	controllers.GlobalHubPolicyNamespaceLabel: "default", // global-hub
			// },
		},
		Spec: policyv1.PolicySpec{
			Disabled:        true,
			PolicyTemplates: make([]*policyv1.PolicyTemplate, 0),
		},
		Status: policyv1.PolicyStatus{
			Status: make([]*policyv1.CompliancePerClusterStatus, 0),
		},
	}

	unMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(policy1)
	if err != nil {
		t.Error(err)
	}
	if _, err = client.Resource(policyGVR).Namespace(policy1.Namespace).Create(context.TODO(), &unstructured.Unstructured{Object: unMap}, metav1.CreateOptions{}); err != nil {
		t.Error(err)
	}

	// 3. reconcile policy to add global hub label
	unObj, err := client.Resource(policyGVR).Namespace(policy1.Namespace).Get(context.TODO(), policy1.Name, metav1.GetOptions{})
	if err != nil {
		t.Error(err)
	}
	if err := reconcileFunc(context.TODO(), unObj); err != nil {
		t.Fatal(fmt.Errorf("error to reconcile policy: %w", err))
	}

	labeledPolicy := &policyv1.Policy{}
	if err := getPolicyWithSummary(policy1.Namespace, policy1.Name, labeledPolicy); err != nil {
		t.Error(err)
	}
	if labeledPolicy.GetLabels()[controllers.GlobalHubPolicyNamespaceLabel] != policy1.GetNamespace() {
		t.Errorf("should add global label to resource %s/%s", policy1.Namespace, policy1.Name)
	}

	// 4. reconcile global hub policy test-policy from syncer's policy
	labeledPolicy.Status.Status = append(labeledPolicy.Status.Status, &policyv1.CompliancePerClusterStatus{
		ComplianceState:  policyv1.Compliant,
		ClusterName:      "cluster1",
		ClusterNamespace: "default",
	})
	labeledPolicy.Status.Status = append(labeledPolicy.Status.Status, &policyv1.CompliancePerClusterStatus{
		ComplianceState:  policyv1.NonCompliant,
		ClusterName:      "cluster2",
		ClusterNamespace: "default",
	})

	unMap, err = runtime.DefaultUnstructuredConverter.ToUnstructured(labeledPolicy)
	if err != nil {
		t.Error(err)
	}
	unObj, err = client.Resource(policyGVR).Namespace(labeledPolicy.Namespace).UpdateStatus(context.TODO(), &unstructured.Unstructured{Object: unMap}, metav1.UpdateOptions{})
	if err != nil {
		t.Error(err)
	}

	// 3. reconcile policySummary
	if err := reconcileFunc(context.TODO(), unObj); err != nil {
		t.Fatal(fmt.Errorf("error to reconcile policy: %w", err))
	}

	// 4. verify the reconcile policy
	summariedPolicy := &policyv1.Policy{}
	if err := getPolicyWithSummary(policy1.Namespace, policy1.Name, summariedPolicy); err != nil {
		t.Fatal(fmt.Errorf("error to get the reconciled policy: %w", err))
	}
	t.Log(prettyPrint(summariedPolicy))
	// if newPolicy.Status.ComplianceSummary.Compliant != 1 || newPolicy.Status.ComplianceSummary.NonCompliant != 1 {
	// 	t.Fatal(fmt.Errorf("compliance summary is incorrect: %s", prettyPrint(newPolicy)))
	// }
}

func getPolicyWithSummary(namespace, name string, policy *policyv1.Policy) error {
	unObj, err := client.Resource(policyGVR).Namespace(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(unObj.UnstructuredContent(), policy)
	if err != nil {
		return err
	}
	return nil
}

func createNamespace(ctx context.Context, name string) error {
	gvr := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "namespaces",
	}

	newNamespace := &unstructured.Unstructured{}
	newNamespace.SetAPIVersion("v1")
	newNamespace.SetKind("Namespace")
	newNamespace.SetName(name)

	if _, err := client.Resource(gvr).Create(ctx, newNamespace, metav1.CreateOptions{}); err != nil {
		if !errors.IsAlreadyExists(err) {
			return err
		}
	}
	return nil
}

func prettyPrint(obj any) string {
	bytes, _ := json.MarshalIndent(obj, "", "  ")
	return string(bytes)
}
