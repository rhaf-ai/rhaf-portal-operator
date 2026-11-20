// Copyright 2026 The RHAF Portal Operator Authors
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"testing"
	"time"

	rhafv1alpha1 "github.com/rhaf-ai/rhaf-portal-operator/api/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestReconcileFinalizer(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := rhafv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("add RHAF API to scheme: %v", err)
	}

	t.Run("adds finalizer", func(t *testing.T) {
		core := testCore()
		reconciler, kubeClient := newCoreReconciler(t, core)

		if _, err := reconciler.Reconcile(t.Context(), requestFor(core)); err != nil {
			t.Fatalf("reconcile: %v", err)
		}

		actual := &rhafv1alpha1.RHAFPortalCore{}
		if err := kubeClient.Get(t.Context(), client.ObjectKey{Name: core.Name}, actual); err != nil {
			t.Fatalf("get core: %v", err)
		}
		if !containsString(actual.Finalizers, finalizerName) {
			t.Fatalf("finalizers = %v, want %q", actual.Finalizers, finalizerName)
		}
	})

	t.Run("removes finalizer on deletion", func(t *testing.T) {
		now := metav1.NewTime(time.Now())
		server := &rhafv1alpha1.RHAFMCPServer{ObjectMeta: metav1.ObjectMeta{
			Name:              "server",
			Namespace:         "tools",
			Finalizers:        []string{finalizerName},
			DeletionTimestamp: &now,
		}}
		kubeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(server).Build()
		reconciler := &RHAFMCPServerReconciler{Client: kubeClient}

		if _, err := reconciler.Reconcile(t.Context(), requestFor(server)); err != nil {
			t.Fatalf("reconcile: %v", err)
		}

		actual := &rhafv1alpha1.RHAFMCPServer{}
		err := kubeClient.Get(t.Context(), client.ObjectKeyFromObject(server), actual)
		if apierrors.IsNotFound(err) {
			return
		}
		if err != nil {
			t.Fatalf("get server after deletion: %v", err)
		}
		if containsString(actual.Finalizers, finalizerName) {
			t.Fatalf("finalizers = %v, do not want %q", actual.Finalizers, finalizerName)
		}
	})
}

func requestFor(object client.Object) ctrl.Request {
	return ctrl.Request{NamespacedName: types.NamespacedName{
		Name:      object.GetName(),
		Namespace: object.GetNamespace(),
	}}
}
