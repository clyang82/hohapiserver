package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	toolscache "k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"
	policyv1 "open-cluster-management.io/governance-policy-propagator/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	policysummaryv1alpha1 "github.com/clyang82/multicluster-global-hub-lite/apis/policysummary/v1alpha1"
)

type PolicyController struct {
	context context.Context
	name    string
	// client is used to apply resources
	client dynamic.Interface
	// informer is used to watch the hosted resources
	informer cache.Informer

	gvr schema.GroupVersionResource

	queue workqueue.RateLimitingInterface
	cache cache.Cache

	createInstance func() client.Object

	Controller
}

func NewPolicyController(ctx context.Context, name string, client dynamic.Interface,
	gvr schema.GroupVersionResource, informer cache.Informer, cache cache.Cache, createInstance func() client.Object,
) *PolicyController {
	queue := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), name)
	c := &PolicyController{
		context:        ctx,
		name:           name,
		client:         client,
		gvr:            gvr,
		informer:       informer,
		queue:          queue,
		cache:          cache,
		createInstance: createInstance,
	}

	c.informer.AddEventHandler(
		toolscache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				c.enqueue(obj)
			},
			UpdateFunc: func(_, obj interface{}) {
				c.enqueue(obj)
			},
			DeleteFunc: func(obj interface{}) {
				c.enqueue(obj)
			},
		},
	)
	return c
}

// enqueue enqueues a resource.
func (c *PolicyController) enqueue(obj interface{}) {
	key, err := toolscache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.queue.Add(key)
}

func (c *PolicyController) Run(ctx context.Context, numThreads int) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	klog.Infof("Starting %s controller", c.name)
	defer klog.Infof("Shutting down %s controller", c.name)

	for i := 0; i < numThreads; i++ {
		go wait.UntilWithContext(ctx, c.startWorker, time.Second)
	}

	<-ctx.Done()
}

func (c *PolicyController) startWorker(ctx context.Context) {
	for c.processNextWorkItem(ctx) {
	}
}

func (c *PolicyController) processNextWorkItem(ctx context.Context) bool {
	// Wait until there is a new item in the working queue
	k, quit := c.queue.Get()
	if quit {
		return false
	}
	key := k.(string)

	// No matter what, tell the queue we're done with this key, to unblock
	// other workers.
	defer c.queue.Done(key)

	if err := c.process(ctx, key); err != nil {
		utilruntime.HandleError(fmt.Errorf("%q controller failed to sync %q, err: %w", c.name, key, err))
		c.queue.AddRateLimited(key)
		return true
	}
	c.queue.Forget(key)
	return true
}

func (c *PolicyController) process(ctx context.Context, key string) error {
	namespace, name, err := toolscache.SplitMetaNamespaceKey(key)
	if err != nil {
		klog.Errorf("invalid key: %q: %v", key, err)
		return nil
	}

	klog.V(5).Infof("process object is: %s/%s", namespace, name)
	instance := c.createInstance()
	err = c.cache.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, instance)
	if err != nil {
		return err
	}

	klog.V(5).Infof("process object is: %v", instance)
	err = c.Reconcile(ctx, instance)
	if err != nil {
		return err
	}

	return nil
}

func (c *PolicyController) Reconcile(ctx context.Context, obj interface{}) error {
	klog.Info("Starting to reconcile the policy")
	data, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return err
	}
	klog.Infof("the instance %v \n", string(data))
	policy := &policyv1.Policy{}
	if err := json.Unmarshal(data, policy); err != nil {
		return err
	}

	// TODO get the latest policy
	klog.Infof("get the policy %s/%s \n", policy.GetNamespace(), policy.GetName())
	unStructObj, err := c.client.Resource(policyv1.GroupVersion.WithResource("policies")).Namespace(policy.GetNamespace()).Get(ctx, policy.GetName(), metav1.GetOptions{})
	if errors.IsNotFound(err) {
		klog.Warningf("policy is not found %s/%s \n", policy.GetNamespace(), policy.GetName())
		return nil
	}
	if err != nil {
		return err
	}

	newPolicy := &policyv1.Policy{}
	if err = runtime.DefaultUnstructuredConverter.FromUnstructured(unStructObj.UnstructuredContent(), newPolicy); err != nil {
		return err
	}
	data, err = json.MarshalIndent(newPolicy, "", "  ")
	if err != nil {
		return err
	}
	klog.Infof("get the policy %v \n", string(data))

	unStructObj, err = c.client.Resource(policysummaryv1alpha1.SchemeBuilder.GroupVersion.WithResource("policysummaries")).Get(ctx, policy.GetName(), metav1.GetOptions{})

	policySummary := &policysummaryv1alpha1.PolicySummary{}
	if err == nil {
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(unStructObj.Object, policySummary)
		if err != nil {
			return err
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
			return err
		}
		unStructObj := &unstructured.Unstructured{Object: unStructMap}
		_, err = c.client.Resource(policysummaryv1alpha1.SchemeBuilder.GroupVersion.WithResource("policysummaries")).Create(ctx, unStructObj, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	policyStatus := policy.Status.Status
	if policyStatus == nil {
		klog.Info("policy with empty status")
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

	if policySummary.Status.RegionalHubs == nil {
		policySummary.Status = policysummaryv1alpha1.PolicySummaryStatus{
			Compliant:    0,
			NonCompliant: 0,
			RegionalHubs: []policysummaryv1alpha1.RegionalHubPolicyStatus{
				{
					Name:         policy.GetNamespace(),
					Compliant:    0,
					NonCompliant: 0,
				},
			},
		}
	}

	exist := false
	for _, regionalHub := range policySummary.Status.RegionalHubs {
		if newRegionalHub.Name == regionalHub.Name {
			exist = true
			deltaCompliant := regionalHub.Compliant - newRegionalHub.Compliant
			deltaNonCompliant := regionalHub.NonCompliant - newRegionalHub.NonCompliant
			policySummary.Status.Compliant -= deltaCompliant
			policySummary.Status.NonCompliant -= deltaNonCompliant
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
	unStructObj = &unstructured.Unstructured{Object: unStructMap}
	if _, err := c.client.Resource(policysummaryv1alpha1.SchemeBuilder.GroupVersion.WithResource("policysummaries")).Update(ctx, unStructObj, metav1.UpdateOptions{}); err != nil {
		return err
	}
	return nil
}
