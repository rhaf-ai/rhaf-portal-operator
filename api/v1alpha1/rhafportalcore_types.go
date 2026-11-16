// Copyright 2026 The RHAF Portal Operator Authors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RHAFPortalCoreSpec defines the desired state of a RHAF Portal Core installation.
type RHAFPortalCoreSpec struct {
	// Image is the Core runtime container image to install.
	// +required
	// +kubebuilder:validation:MinLength=1
	Image string `json:"image"`

	// Resources configures CPU and memory requests and limits for the Core runtime.
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// Storage configures the persistent data volume used by the Core runtime.
	// +optional
	Storage RHAFPortalCoreStorageSpec `json:"storage,omitempty"`
}

// RHAFPortalCoreStorageSpec configures persistent storage for Core.
type RHAFPortalCoreStorageSpec struct {
	// Size is the requested persistent-volume size, such as 10Gi.
	// +optional
	Size resource.Quantity `json:"size,omitempty"`

	// StorageClassName selects the persistent-volume storage class.
	// +optional
	StorageClassName string `json:"storageClassName,omitempty"`
}

// RHAFPortalCoreStatus defines the observed state of RHAFPortalCore.
type RHAFPortalCoreStatus struct {
	// ObservedGeneration is the most recent generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions report configuration validity, registration, freshness, upstream
	// availability, presentation readiness, and (when configured) OLS state.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// Condition type names shared by RHAF resources.
const (
	ConditionConfigValid       = "ConfigValid"
	ConditionRegistered        = "Registered"
	ConditionFresh             = "Fresh"
	ConditionUpstreamAvailable = "UpstreamAvailable"
	ConditionPresentationReady = "PresentationReady"
	ConditionOLSReady          = "OLSReady"
)

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Namespaced,shortName=rhafcore
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="PresentationReady")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// RHAFPortalCore is the namespaced installation resource for RHAF Portal Core.
type RHAFPortalCore struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RHAFPortalCoreSpec   `json:"spec"`
	Status RHAFPortalCoreStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RHAFPortalCoreList contains a list of RHAFPortalCore resources.
type RHAFPortalCoreList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []RHAFPortalCore `json:"items"`
}
