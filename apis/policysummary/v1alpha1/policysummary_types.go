/*
Copyright 2022.

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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// PolicySummarySpec defines the desired state of PolicySummary
type PolicySummarySpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// PolicySummaryStatus defines the observed state of PolicySummary
type PolicySummaryStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	// +kubebuilder:default:=0
	Compliant uint32 `json:"compliant"`
	// +kubebuilder:default:=0
	NonCompliant uint32                    `json:"noncompliant"`
	RegionalHubs []RegionalHubPolicyStatus `json:"regionalHubs,omitempty"`
}

type RegionalHubPolicyStatus struct {
	Name string `json:"name,omitempty"`
	// +kubebuilder:default:=0
	Compliant uint32 `json:"compliant"`
	// +kubebuilder:default:=0
	NonCompliant uint32 `json:"noncompliant"`
}

// PolicySummary is the Schema for the policysummaries API

//+k8s:openapi-gen=true
//+kubebuilder:object:root=true
//+kubebuilder:resource:scope="Cluster",shortName={"plcs"}
//+kubebuilder:subresource:status
type PolicySummary struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PolicySummarySpec   `json:"spec,omitempty"`
	Status PolicySummaryStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// PolicySummaryList contains a list of PolicySummary
type PolicySummaryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PolicySummary `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PolicySummary{}, &PolicySummaryList{})
}
