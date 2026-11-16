// Copyright 2026 The RHAF Portal Operator Authors
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"

	rhafv1alpha1 "github.com/rhaf-ai/rhaf-portal-operator/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// RHAFMCPServerReconciler reconciles a RHAFMCPServer resource.
type RHAFMCPServerReconciler struct {
	client.Client
}

// +kubebuilder:rbac:groups=rhaf.ai.redhat.com,resources=rhafmcpservers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rhaf.ai.redhat.com,resources=rhafmcpservers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=rhaf.ai.redhat.com,resources=rhafmcpservers/finalizers,verbs=update

// Reconcile maintains the resource finalizer. Core registration is added separately.
func (r *RHAFMCPServerReconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	server := &rhafv1alpha1.RHAFMCPServer{}
	if err := r.Get(ctx, request.NamespacedName, server); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	return ctrl.Result{}, reconcileFinalizer(ctx, r.Client, server)
}

// SetupWithManager registers the controller with the manager.
func (r *RHAFMCPServerReconciler) SetupWithManager(manager ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(manager).For(&rhafv1alpha1.RHAFMCPServer{}).Complete(r)
}
