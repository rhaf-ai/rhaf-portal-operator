// Copyright 2026 The RHAF Portal Operator Authors
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"

	rhafv1alpha1 "github.com/rhaf-ai/rhaf-portal-operator/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// RHAFPortalCoreReconciler reconciles a RHAFPortalCore resource.
type RHAFPortalCoreReconciler struct {
	client.Client
}

// +kubebuilder:rbac:groups=rhaf.ai.redhat.com,resources=rhafportalcores,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rhaf.ai.redhat.com,resources=rhafportalcores/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=rhaf.ai.redhat.com,resources=rhafportalcores/finalizers,verbs=update

// Reconcile maintains the resource finalizer. Workload reconciliation is added separately.
func (r *RHAFPortalCoreReconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	core := &rhafv1alpha1.RHAFPortalCore{}
	if err := r.Get(ctx, request.NamespacedName, core); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	return ctrl.Result{}, reconcileFinalizer(ctx, r.Client, core)
}

// SetupWithManager registers the controller with the manager.
func (r *RHAFPortalCoreReconciler) SetupWithManager(manager ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(manager).For(&rhafv1alpha1.RHAFPortalCore{}).Complete(r)
}
