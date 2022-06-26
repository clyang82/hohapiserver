package syncer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"
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

const statusSyncerAgent = "hoh#status-syncer/v0.0.0"

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
	for i := 1; i <= 100000; i++ {
		upstreamObj := downstreamObj.DeepCopy()
		upstreamObj.SetUID("")
		upstreamObj.SetResourceVersion("")
		upstreamObj.SetNamespace(upstreamNamespace)
		upstreamObj.SetName(fmt.Sprintf("%s-%v", upstreamObj.GetName(), i))

		var existing *unstructured.Unstructured
		var err error
		for {
			existing, err = c.toClient.Resource(gvr).Namespace(upstreamNamespace).Get(ctx, upstreamObj.GetName(), metav1.GetOptions{})
			if err != nil {
				if gvr.Resource == "managedclusters" && errors.IsNotFound(err) {
					_, err = c.applyToUpstream(ctx, gvr, upstreamNamespace, downstreamObj)
					if err != nil {
						klog.Infof("Error upserting %s %s from downstream %s: %v", gvr.Resource, upstreamObj.GetName(), downstreamObj.GetName(), err)
						time.Sleep(time.Second)
					}
				} else {
					klog.Errorf("Getting resource %s/%s: %v", upstreamNamespace, upstreamObj.GetName(), err)
					break
				}
			} else {
				break
			}
		}

		upstreamObj.SetResourceVersion(existing.GetResourceVersion())
		if _, err := c.toClient.Resource(gvr).Namespace(upstreamNamespace).UpdateStatus(ctx, upstreamObj, metav1.UpdateOptions{}); err != nil {
			klog.Errorf("Failed updating status of resource %s/%s from leaf hub cluster namespace %s: %v", upstreamNamespace, upstreamObj.GetName(), downstreamObj.GetNamespace(), err)
			return err
		}
		klog.Infof("Updated status of resource %s/%s from leaf hub cluster namespace %s", upstreamNamespace, upstreamObj.GetName(), downstreamObj.GetNamespace())
	}
	return nil
}

// applyToUpstream is used to apply managedclusters to upstream
func (c *Controller) applyToUpstream(ctx context.Context, gvr schema.GroupVersionResource, upstreamNamespace string, downstreamObj *unstructured.Unstructured) (*unstructured.Unstructured, error) {

	upstreamObj := downstreamObj.DeepCopy()
	upstreamObj.SetUID("")
	upstreamObj.SetResourceVersion("")
	upstreamObj.SetManagedFields(nil)
	upstreamObj.SetClusterName("")
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
		return nil, err
	}

	return c.toClient.Resource(gvr).Patch(ctx, upstreamObj.GetName(), types.ApplyPatchType, data, metav1.PatchOptions{FieldManager: syncerApplyManager, Force: pointer.Bool(true)})
}
