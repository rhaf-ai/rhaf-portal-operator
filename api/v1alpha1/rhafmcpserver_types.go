// Copyright 2026 The RHAF Portal Operator Authors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// NamespaceBasePattern is the validation expression for NamespaceBase.
const NamespaceBasePattern = `^[a-z0-9](?:[a-z0-9-]{0,49}[a-z0-9])?$`

// RHAFMCPServerSpec defines an existing MCP endpoint declared to RHAF Portal Core.
type RHAFMCPServerSpec struct {
	// Address is the endpoint address reachable by Core.
	// +required
	// +kubebuilder:validation:MinLength=1
	Address string `json:"address"`

	// NamespaceBase scopes names supplied by the endpoint. It must have 1 to 51
	// lowercase letters, digits, or hyphens and cannot start or end with a hyphen.
	// +required
	// +kubebuilder:validation:Pattern=`^[a-z0-9](?:[a-z0-9-]{0,49}[a-z0-9])?$`
	NamespaceBase string `json:"namespaceBase"`

	// Labels are metadata supplied with the endpoint declaration.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// DiscoveryCredentialBindingID identifies the credential binding used for
	// endpoint discovery.
	// +optional
	DiscoveryCredentialBindingID string `json:"discoveryCredentialBindingID,omitempty"`

	// InvocationCredentialBindingID identifies the credential binding used for
	// endpoint invocation.
	// +optional
	InvocationCredentialBindingID string `json:"invocationCredentialBindingID,omitempty"`
}

// RHAFMCPServerStatus defines the observed state of RHAFMCPServer.
type RHAFMCPServerStatus struct {
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

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Namespaced,shortName=rhafmcp
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Registered",type=string,JSONPath=`.status.conditions[?(@.type=="Registered")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// RHAFMCPServer declares an existing MCP server in the Core installation namespace.
type RHAFMCPServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RHAFMCPServerSpec   `json:"spec"`
	Status RHAFMCPServerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RHAFMCPServerList contains a list of RHAFMCPServer resources.
type RHAFMCPServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []RHAFMCPServer `json:"items"`
}
