// Copyright 2025 Butler Labs
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

// StewardControlPlaneTemplateSpec defines the desired state of StewardControlPlaneTemplate.
type StewardControlPlaneTemplateSpec struct {
	Template StewardControlPlaneTemplateResource `json:"template"`
}

//+kubebuilder:object:root=true
//+kubebuilder:resource:categories=cluster-api;steward,shortName=stcpt

// StewardControlPlaneTemplate is the Schema for the stewardcontrolplanetemplates API.
type StewardControlPlaneTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec StewardControlPlaneTemplateSpec `json:"spec,omitempty"`
}

//+kubebuilder:object:root=true

// StewardControlPlaneTemplateList contains a list of StewardControlPlaneTemplate.
type StewardControlPlaneTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []StewardControlPlaneTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&StewardControlPlaneTemplate{}, &StewardControlPlaneTemplateList{})
}

// StewardControlPlaneTemplateResource describes the data needed to create a StewardControlPlane from a template.
type StewardControlPlaneTemplateResource struct {
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	ObjectMeta clusterv1.ObjectMeta     `json:"metadata,omitempty"`
	Spec       StewardControlPlaneFields `json:"spec"`
}
