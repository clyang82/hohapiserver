package controllers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	policyv1 "open-cluster-management.io/governance-policy-propagator/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	policysummaryv1alpha1 "github.com/clyang82/multicluster-global-hub-lite/apis/policysummary/v1alpha1"
	"github.com/clyang82/multicluster-global-hub-lite/server/controllers"
)

var (
	cfg              *rest.Config
	client           dynamic.Interface
	policySummaryGVR schema.GroupVersionResource
)

func TestMain(m *testing.M) {
	policySummaryGVR = policysummaryv1alpha1.SchemeBuilder.GroupVersion.WithResource("policysummaries")

	// start testEnv
	testEnv := &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "manifests"),
		},
	}
	var err error
	if err != nil {
		panic(err)
	}

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

	// 2. create policy test-policy in ns1
	policy1 := &policyv1.Policy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-policy",
			Namespace: "ns1",
		},
		Status: policyv1.PolicyStatus{
			Status: make([]*policyv1.CompliancePerClusterStatus, 0),
		},
	}
	policy1.Status.Status = append(policy1.Status.Status, &policyv1.CompliancePerClusterStatus{
		ComplianceState:  policyv1.Compliant,
		ClusterName:      "cluster1",
		ClusterNamespace: "default",
	})
	policy1.Status.Status = append(policy1.Status.Status, &policyv1.CompliancePerClusterStatus{
		ComplianceState:  policyv1.NonCompliant,
		ClusterName:      "cluster2",
		ClusterNamespace: "default",
	})

	// 3. reconcile policySummary
	unStructMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(policy1)
	if err != nil {
		panic(err)
	}
	if err := reconcileFunc(context.TODO(), &unstructured.Unstructured{Object: unStructMap}); err != nil {
		t.Fatal(fmt.Errorf("error to reconcile policySummary: %w", err))
	}

	// 4. verify policySummary
	ps, err := getPolicySummary(policy1.GetName())
	if err != nil {
		t.Fatal(fmt.Errorf("error the policySummary: %w", err))
	}
	if ps.Status.Compliant != 1 || ps.Status.NonCompliant != 1 {
		t.Fatalf("policySummary with incorrect status %v", ps.Status)
	}

	// 5. sync another regional hub policy status
	status3 := &policyv1.CompliancePerClusterStatus{
		ComplianceState:  policyv1.Compliant,
		ClusterName:      "cluster1",
		ClusterNamespace: "default",
	}
	statuses2 := make([]*policyv1.CompliancePerClusterStatus, 0)
	statuses2 = append(statuses2, status3)
	policy2 := &policyv1.Policy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-policy",
			Namespace: "ns2",
		},
		Status: policyv1.PolicyStatus{
			Status: statuses2,
		},
	}
	unStructMap, err = runtime.DefaultUnstructuredConverter.ToUnstructured(policy2)
	if err != nil {
		panic(err)
	}
	if err := reconcileFunc(context.TODO(), &unstructured.Unstructured{Object: unStructMap}); err != nil {
		t.Error(err)
	}

	// 6. verify policySummary
	ps, err = getPolicySummary(policy1.GetName())
	if err != nil {
		t.Fatal(fmt.Errorf("error the policySummary: %w", err))
	}
	if ps.Status.Compliant != 2 || ps.Status.NonCompliant != 1 {
		t.Fatalf("policySummary with incorrect status %v", ps.Status)
	}

	t.Log(prettyPrint(ps))
}

func getPolicySummary(name string) (*policysummaryv1alpha1.PolicySummary, error) {
	unObj, err := client.Resource(policySummaryGVR).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	policySummary := &policysummaryv1alpha1.PolicySummary{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(unObj.UnstructuredContent(), policySummary)
	if err != nil {
		return nil, err
	}
	return policySummary, nil
}

func prettyPrint(obj any) string {
	bytes, _ := json.MarshalIndent(obj, "", "  ")
	return string(bytes)
}
