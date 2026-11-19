// Copyright 2026 The RHAF Portal Operator Authors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Default Core listener ports and storage size.
const (
	DefaultManagementPort int32 = 8080
	DefaultMCPPort        int32 = 8081
	DefaultStorageSize          = "100Mi"
)

// RHAFPortalCoreSpec defines the desired state of a RHAF Portal Core installation.
// +kubebuilder:validation:XValidation:rule="self.management.port != self.mcp.port",message="management and mcp ports must differ"
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
	// +kubebuilder:default={}
	Storage RHAFPortalCoreStorageSpec `json:"storage,omitempty"`

	// Management configures the internal management listener.
	// +optional
	// +kubebuilder:default={}
	Management RHAFPortalCoreManagementSpec `json:"management,omitempty"`

	// MCP configures the internal MCP listener.
	// +optional
	// +kubebuilder:default={}
	MCP RHAFPortalCoreMCPSpec `json:"mcp,omitempty"`

	// WanakuConfig references a ConfigMap key with the Wanaku bootstrap YAML.
	// The operator mounts it read-only and passes it to Core with --wanaku-config.
	// +optional
	WanakuConfig *RHAFPortalCoreWanakuConfigSpec `json:"wanakuConfig,omitempty"`

	// PodSecurityContext is applied to the Core pod. Leave it empty on OpenShift,
	// which assigns the UID and fsGroup. On other clusters, set fsGroup (for
	// example 1001) so Core can write to the data volume. Core always runs as a
	// non-root user: the operator sets runAsNonRoot and a RuntimeDefault seccomp
	// profile when they are absent.
	// +optional
	// +kubebuilder:validation:XValidation:rule="!has(self.runAsNonRoot) || self.runAsNonRoot",message="runAsNonRoot cannot be false"
	// +kubebuilder:validation:XValidation:rule="!has(self.runAsUser) || self.runAsUser != 0",message="runAsUser cannot be 0"
	PodSecurityContext *corev1.PodSecurityContext `json:"podSecurityContext,omitempty"`
}

// RHAFPortalCoreManagementSpec configures the Core management listener.
type RHAFPortalCoreManagementSpec struct {
	// Port is the management listener and Service port.
	// +optional
	// +kubebuilder:default=8080
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port,omitempty"`
}

// RHAFPortalCoreMCPSpec configures the Core MCP listener.
type RHAFPortalCoreMCPSpec struct {
	// Port is the MCP listener and Service port.
	// +optional
	// +kubebuilder:default=8081
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port,omitempty"`
}

// RHAFPortalCoreWanakuConfigSpec selects the Wanaku bootstrap configuration source.
type RHAFPortalCoreWanakuConfigSpec struct {
	// ConfigMapRef selects a key of a ConfigMap in the installation namespace.
	// +required
	ConfigMapRef ConfigMapKeyReference `json:"configMapRef"`
}

// ConfigMapKeyReference selects a key of a ConfigMap in the same namespace.
type ConfigMapKeyReference struct {
	// Name is the ConfigMap name.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`
	Name string `json:"name"`

	// Key is the ConfigMap data key that holds the YAML document.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Pattern=`^[-._a-zA-Z0-9]+$`
	Key string `json:"key"`
}

// RHAFPortalCoreStorageSpec configures persistent storage for Core.
type RHAFPortalCoreStorageSpec struct {
	// Size is the requested persistent-volume size, such as 10Gi. The PVC is
	// created once; later changes to this field are not applied.
	// +optional
	// +kubebuilder:default="100Mi"
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
	ConditionCoreAvailable     = "CoreAvailable"
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
