package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// HubControlPlaneSpec defines the desired state of HubControlPlane
type HubControlPlaneSpec struct {
	Endpoint string `json:"endpoint,omitempty"`
}

// HubControlPlaneStatus defines the observed state of HubControlPlane
type HubControlPlaneStatus struct {
	Addons          []string              `json:"addons,omitempty"`
	ManagedClusters ManagedClustersStatus `json:"managedClusters,omitempty"`
}

// ManagedClustersStatus defines managed clusters with available, unavailable and unknown status
type ManagedClustersStatus struct {
	Available   []string `json:"available,omitempty"`
	Unavailable []string `json:"unavailable,omitempty"`
	Unknown     []string `json:"unknown,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Cluster

// HubControlPlane is the Schema for the hubcontrolplanes API
type HubControlPlane struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HubControlPlaneSpec   `json:"spec,omitempty"`
	Status HubControlPlaneStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// HubControlPlaneList contains a list of HubControlPlane
type HubControlPlaneList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HubControlPlane `json:"items"`
}

func init() {
	SchemeBuilder.Register(&HubControlPlane{}, &HubControlPlaneList{})
}
