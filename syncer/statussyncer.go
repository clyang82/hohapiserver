/*
Copyright 2021 The KCP Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package syncer

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"

	hubcontrolplanev1alpha1 "github.com/clyang82/multicluster-global-hub-lite/apis/hubcontrolplane/v1alpha1"
)

var (
	hubControlPlaneGVR        = schema.GroupVersionResource{Group: "cluster.open-cluster-management.io", Version: "v1alpha1", Resource: "hubcontrolplanes"}
	managedClusterGVR         = schema.GroupVersionResource{Group: "cluster.open-cluster-management.io", Version: "v1", Resource: "managedclusters"}
	clusterManagementAddonGVR = schema.GroupVersionResource{Group: "addon.open-cluster-management.io", Version: "v1alpha1", Resource: "clustermanagementaddons"}
)

func deepEqualStatus(oldObj, newObj interface{}) bool {
	oldUnstrob, isOldObjUnstructured := oldObj.(*unstructured.Unstructured)
	newUnstrob, isNewObjUnstructured := newObj.(*unstructured.Unstructured)
	if !isOldObjUnstructured || !isNewObjUnstructured || oldObj == nil || newObj == nil {
		return false
	}

	newStatus := newUnstrob.UnstructuredContent()["status"]
	oldStatus := oldUnstrob.UnstructuredContent()["status"]

	return equality.Semantic.DeepEqual(oldStatus, newStatus)
}

const statusSyncerAgent = "globalhub#status-syncer/v0.0.0"

func NewStatusSyncer(from, to *rest.Config) (*Controller, error) {
	from = rest.CopyConfig(from)
	from.UserAgent = statusSyncerAgent
	to = rest.CopyConfig(to)
	to.UserAgent = statusSyncerAgent

	fromClient := dynamic.NewForConfigOrDie(from)
	toClient := dynamic.NewForConfigOrDie(to)
	toDiscoverClient := discovery.NewDiscoveryClientForConfigOrDie(to)
	toRestMapper := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(toDiscoverClient))

	return New(fromClient, toClient, from, toRestMapper, SyncUp)
}

func (c *Controller) updateStatusInUpstream(ctx context.Context, dr dynamic.ResourceInterface,
	gvr schema.GroupVersionResource, upstreamNamespace string, downstreamObj *unstructured.Unstructured,
) error {
	// for customresourcedefinition and clustermanagementaddon changes, need to translate to hubcontrolplane and then apply to upstream
	if gvr.Resource == "customresourcedefinitions" || gvr.Resource == "clustermanagementaddons" {
		return c.applyHubControlPlaneInUpstream(ctx, gvr, upstreamNamespace, downstreamObj)
	}

	// for managedcluster change, need to apply it to hubcontrolplane and also syncer it to upstream
	if gvr.Resource == "managedclusters" {
		if err := c.applyHubControlPlaneInUpstream(ctx, gvr, upstreamNamespace, downstreamObj); err != nil {
			// print error and continue
			klog.Errorf("Failed to apply hubcontrolplane for managedcluster %s change: %v", downstreamObj.GetName(), err)
		}
	}

	upstreamObj := downstreamObj.DeepCopy()
	upstreamObj.SetUID("")
	upstreamObj.SetResourceVersion("")
	upstreamObj.SetNamespace(upstreamNamespace)

	existing, err := dr.Get(ctx, upstreamObj.GetName(), metav1.GetOptions{})
	if err != nil {
		if gvr.Resource == "managedclusters" && errors.IsNotFound(err) {
			c.applyToUpstream(ctx, dr, gvr, upstreamNamespace, downstreamObj)
			return nil
		}
		klog.Errorf("Getting resource %s/%s: %v", upstreamNamespace, upstreamObj.GetName(), err)
		return err
	}

	upstreamObj.SetResourceVersion(existing.GetResourceVersion())
	if _, err := dr.UpdateStatus(ctx, upstreamObj, metav1.UpdateOptions{}); err != nil {
		klog.Errorf("Failed updating status of resource %s/%s from leaf hub cluster namespace %s: %v", upstreamNamespace, upstreamObj.GetName(), downstreamObj.GetNamespace(), err)
		return err
	}
	klog.Infof("Updated status of resource %s/%s from leaf hub cluster namespace %s", upstreamNamespace, upstreamObj.GetName(), downstreamObj.GetNamespace())

	return nil
}

// applyToUpstream is used to apply managedclusters to upstream
func (c *Controller) applyToUpstream(ctx context.Context, dr dynamic.ResourceInterface, gvr schema.GroupVersionResource, upstreamNamespace string, downstreamObj *unstructured.Unstructured) error {

	upstreamObj := downstreamObj.DeepCopy()
	upstreamObj.SetUID("")
	upstreamObj.SetResourceVersion("")
	upstreamObj.SetManagedFields(nil)
	// Deletion fields are immutable and set by the downstream API server
	upstreamObj.SetDeletionTimestamp(nil)
	upstreamObj.SetDeletionGracePeriodSeconds(nil)
	// Strip owner references, to avoid orphaning by broken references,
	// and make sure cascading deletion is only performed once upstream.
	upstreamObj.SetOwnerReferences(nil)
	// Strip finalizers to avoid the deletion of the downstream resource from being blocked.
	upstreamObj.SetFinalizers(nil)

	// Marshalling the unstructured object is good enough as SSA patch
	data, err := json.Marshal(upstreamObj)
	if err != nil {
		return err
	}

	if _, err := dr.Patch(ctx, upstreamObj.GetName(), types.ApplyPatchType, data, metav1.PatchOptions{FieldManager: syncerApplyManager, Force: pointer.Bool(true)}); err != nil {
		klog.Infof("Error upserting %s %s from downstream %s: %v", gvr.Resource, upstreamObj.GetName(), downstreamObj.GetName(), err)
		return err
	}
	klog.Infof("Upserted %s %s from upstream %s", gvr.Resource, upstreamObj.GetName(), downstreamObj.GetName())

	return nil
}

func (c *Controller) applyHubControlPlaneInUpstream(ctx context.Context, gvr schema.GroupVersionResource, upstreamNamespace string, downstreamObj *unstructured.Unstructured) error {
	syncerNamespace, ok := os.LookupEnv("POD_NAMESPACE")
	if !ok || syncerNamespace == "" {
		return fmt.Errorf("empty environment variable: POD_NAMESPACE")
	}

	managedClusters := []string{}
	for _, managedClusterItem := range c.fromInformers.ForResource(managedClusterGVR).Informer().GetIndexer().List() {
		managedClusterUnstrob, isUnstructured := managedClusterItem.(*unstructured.Unstructured)
		if !isUnstructured {
			return fmt.Errorf("object is expected to be Unstructured, but is %T", managedClusterItem)
		}

		managedClusters = append(managedClusters, managedClusterUnstrob.GetName())
	}

	addons := []string{}
	for _, clusterManagementAddonItem := range c.fromInformers.ForResource(clusterManagementAddonGVR).Informer().GetIndexer().List() {
		clusterManagementAddonUnstrob, isUnstructured := clusterManagementAddonItem.(*unstructured.Unstructured)
		if !isUnstructured {
			return fmt.Errorf("object is expected to be Unstructured, but is %T", clusterManagementAddonItem)
		}

		addons = append(addons, clusterManagementAddonUnstrob.GetName())
	}

	hubControlPlane := &hubcontrolplanev1alpha1.HubControlPlane{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "cluster.open-cluster-management.io/v1alpha1",
			Kind:       "HubControlPlane",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: syncerNamespace,
		},
		Spec: hubcontrolplanev1alpha1.HubControlPlaneSpec{
			Endpoint:        c.fromConfig.Host,
			ManagedClusters: managedClusters,
			Addons:          addons,
		},
	}

	var hubControlPlaneUnstrObj unstructured.Unstructured
	hubControlPlaneUnstrContent, err := runtime.DefaultUnstructuredConverter.ToUnstructured(hubControlPlane)
	if err != nil {
		return fmt.Errorf("failed to convert hubcontrolplane(%s) to unstructured object content: %v", hubControlPlane.GetName(), err)
	}

	hubControlPlaneUnstrObj.SetUnstructuredContent(hubControlPlaneUnstrContent)
	hubControlPlaneUnstrObj.SetGroupVersionKind(hubControlPlane.GetObjectKind().GroupVersionKind())
	c.applyToUpstream(ctx, c.toClient.Resource(hubControlPlaneGVR), hubControlPlaneGVR, "", &hubControlPlaneUnstrObj)

	return nil
}
