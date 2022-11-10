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

	routev1 "github.com/openshift/api/route/v1"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"

	hubcontrolplanev1alpha1 "github.com/clyang82/multicluster-global-hub-lite/apis/hubcontrolplane/v1alpha1"
)

var (
	deployGVR                 = schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	routeGVR                  = schema.GroupVersionResource{Group: "route.openshift.io", Version: "v1", Resource: "routes"}
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

	return New(fromClient, toClient, SyncUp)
}

func (c *Controller) updateStatusInUpstream(ctx context.Context, gvr schema.GroupVersionResource, upstreamNamespace string, downstreamObj *unstructured.Unstructured) error {
	// for route and deployment, need to translate to hubcontrolplane and then apply to upstream
	if gvr.Resource == "routes" || gvr.Resource == "deployments" {
		return c.applyHubControlPlaneInUpstream(ctx, gvr, upstreamNamespace, downstreamObj)
	}

	upstreamObj := downstreamObj.DeepCopy()
	upstreamObj.SetUID("")
	upstreamObj.SetResourceVersion("")
	upstreamObj.SetNamespace(upstreamNamespace)

	existing, err := c.toClient.Resource(gvr).Namespace(upstreamNamespace).Get(ctx, upstreamObj.GetName(), metav1.GetOptions{})
	if err != nil {
		if gvr.Resource == "managedclusters" && errors.IsNotFound(err) {
			c.applyToUpstream(ctx, gvr, upstreamNamespace, downstreamObj)
			return nil
		}
		klog.Errorf("Getting resource %s/%s: %v", upstreamNamespace, upstreamObj.GetName(), err)
		return err
	}

	upstreamObj.SetResourceVersion(existing.GetResourceVersion())
	if _, err := c.toClient.Resource(gvr).Namespace(upstreamNamespace).UpdateStatus(ctx, upstreamObj, metav1.UpdateOptions{}); err != nil {
		klog.Errorf("Failed updating status of resource %s/%s from leaf hub cluster namespace %s: %v", upstreamNamespace, upstreamObj.GetName(), downstreamObj.GetNamespace(), err)
		return err
	}
	klog.Infof("Updated status of resource %s/%s from leaf hub cluster namespace %s", upstreamNamespace, upstreamObj.GetName(), downstreamObj.GetNamespace())

	return nil
}

// applyToUpstream is used to apply managedclusters to upstream
func (c *Controller) applyToUpstream(ctx context.Context, gvr schema.GroupVersionResource, upstreamNamespace string, downstreamObj *unstructured.Unstructured) error {

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

	if _, err := c.toClient.Resource(gvr).Patch(ctx, upstreamObj.GetName(), types.ApplyPatchType, data, metav1.PatchOptions{FieldManager: syncerApplyManager, Force: pointer.Bool(true)}); err != nil {
		klog.Infof("Error upserting %s %s from downstream %s: %v", gvr.Resource, upstreamObj.GetName(), downstreamObj.GetName(), err)
		return err
	}
	klog.Infof("Upserted %s %s from upstream %s", gvr.Resource, upstreamObj.GetName(), downstreamObj.GetName())

	return nil
}

func (c *Controller) applyHubControlPlaneInUpstream(ctx context.Context, gvr schema.GroupVersionResource, upstreamNamespace string, downstreamObj *unstructured.Unstructured) error {
	ocmCPDeploy, ocmCPRoute := &appsv1.Deployment{}, &routev1.Route{}
	if gvr.Resource == "deployments" {
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(downstreamObj.UnstructuredContent(), ocmCPDeploy); err != nil {
			return fmt.Errorf("failed to convert unstructured(%s/%s) to deployment object: %v", downstreamObj.GetNamespace(), downstreamObj.GetName(), err)
		}

		obj, exists, err := c.fromInformers.ForResource(routeGVR).Informer().GetIndexer().GetByKey(fmt.Sprintf("%s/%s", ocmCPDeploy.GetNamespace(), ocmCPDeploy.GetName()))
		if !exists {
			return fmt.Errorf("failed to get route for ocm controlplane route because it doesn't exist.")
		} else if err != nil {
			return fmt.Errorf("failed to get route for ocm controlplane route: %v", err)
		}

		routeUnstrObj, isUnstructured := obj.(*unstructured.Unstructured)
		if !isUnstructured {
			return fmt.Errorf("object to synchronize is expected to be Unstructured, but is %T", obj)
		}

		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(routeUnstrObj.UnstructuredContent(), ocmCPRoute); err != nil {
			return fmt.Errorf("failed to convert unstructured(%s/%s) to route object: %v", routeUnstrObj.GetNamespace(), routeUnstrObj.GetName(), err)
		}
	}
	if gvr.Resource == "routes" {
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(downstreamObj.UnstructuredContent(), ocmCPRoute); err != nil {
			return fmt.Errorf("failed to convert unstructured(%s/%s) to route: %v", downstreamObj.GetNamespace(), downstreamObj.GetName(), err)
		}

		obj, exists, err := c.fromInformers.ForResource(deployGVR).Informer().GetIndexer().GetByKey(fmt.Sprintf("%s/%s", ocmCPRoute.GetNamespace(), ocmCPRoute.GetName()))
		if !exists {
			return fmt.Errorf("failed to get route for ocm controlplane deployment because it doesn't exist.")
		} else if err != nil {
			return fmt.Errorf("failed to get route for ocm controlplane deployment: %v", err)
		}

		deployUnstrObj, isUnstructured := obj.(*unstructured.Unstructured)
		if !isUnstructured {
			return fmt.Errorf("object to synchronize is expected to be Unstructured, but is %T", obj)
		}

		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(deployUnstrObj.UnstructuredContent(), ocmCPDeploy); err != nil {
			return fmt.Errorf("failed to convert unstructured(%s/%s) to route object: %v", deployUnstrObj.GetNamespace(), deployUnstrObj.GetName(), err)
		}
	}

	endpoint := ""
	if len(ocmCPRoute.Status.Ingress) > 0 {
		endpoint = ocmCPRoute.Status.Ingress[0].Host
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
			Name: upstreamNamespace,
		},
		Spec: hubcontrolplanev1alpha1.HubControlPlaneSpec{
			Endpoint:        endpoint,
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
	c.applyToUpstream(ctx, hubControlPlaneGVR, "", &hubControlPlaneUnstrObj)

	return nil
}
