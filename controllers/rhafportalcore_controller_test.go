// Copyright 2026 The RHAF Portal Operator Authors
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"testing"
	"time"

	rhafv1alpha1 "github.com/rhaf-ai/rhaf-portal-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

func coreTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	if err := rhafv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("add RHAF API to scheme: %v", err)
	}

	return scheme
}

func newCoreReconciler(t *testing.T, objects ...client.Object) (*RHAFPortalCoreReconciler, client.Client) {
	t.Helper()
	kubeClient := fake.NewClientBuilder().
		WithScheme(coreTestScheme(t)).
		WithObjects(objects...).
		WithStatusSubresource(&rhafv1alpha1.RHAFPortalCore{}).
		Build()

	return &RHAFPortalCoreReconciler{Client: kubeClient, APIReader: kubeClient, WorkloadNamespace: "test"}, kubeClient
}

func reconcileCore(t *testing.T, reconciler *RHAFPortalCoreReconciler, core *rhafv1alpha1.RHAFPortalCore) {
	t.Helper()
	if _, err := reconciler.Reconcile(t.Context(), requestFor(core)); err != nil {
		t.Fatalf("reconcile %s: %v", core.Name, err)
	}
}

func getCore(t *testing.T, kubeClient client.Client, name string) *rhafv1alpha1.RHAFPortalCore {
	t.Helper()
	core := &rhafv1alpha1.RHAFPortalCore{}
	if err := kubeClient.Get(t.Context(), client.ObjectKey{Name: name}, core); err != nil {
		t.Fatalf("get core %s: %v", name, err)
	}

	return core
}

func assertCondition(t *testing.T, core *rhafv1alpha1.RHAFPortalCore, conditionType string, status metav1.ConditionStatus, reason string) {
	t.Helper()
	condition := meta.FindStatusCondition(core.Status.Conditions, conditionType)
	if condition == nil || condition.Status != status || condition.Reason != reason {
		t.Fatalf("%s condition = %+v, want status %s reason %s", conditionType, condition, status, reason)
	}
}

func exists(t *testing.T, kubeClient client.Client, name string, object client.Object) bool {
	t.Helper()
	err := kubeClient.Get(t.Context(), types.NamespacedName{Name: name, Namespace: "test"}, object)
	if apierrors.IsNotFound(err) {
		return false
	}
	if err != nil {
		t.Fatalf("get %s: %v", name, err)
	}

	return true
}

func TestReconcileCoreCreatesWorkload(t *testing.T) {
	core := testCore()
	reconciler, kubeClient := newCoreReconciler(t, core)

	reconcileCore(t, reconciler, core)
	reconcileCore(t, reconciler, core) // repeatable

	deployment := &appsv1.Deployment{}
	if !exists(t, kubeClient, "rhaf-core", deployment) {
		t.Fatal("deployment rhaf-core not created")
	}
	if owner := metav1.GetControllerOf(deployment); owner == nil || owner.UID != core.UID {
		t.Errorf("deployment controller owner = %v, want core", owner)
	}
	for _, name := range []string{"rhaf-core-mcp", "rhaf-core-management"} {
		service := &corev1.Service{}
		if !exists(t, kubeClient, name, service) {
			t.Fatalf("service %s not created", name)
		}
		if metav1.GetControllerOf(service) == nil {
			t.Errorf("service %s has no controller owner", name)
		}
	}
	pvc := &corev1.PersistentVolumeClaim{}
	if !exists(t, kubeClient, "rhaf-core-data", pvc) {
		t.Fatal("pvc rhaf-core-data not created")
	}
	if len(pvc.OwnerReferences) != 0 {
		t.Errorf("pvc owner references = %v, want none", pvc.OwnerReferences)
	}

	services := &corev1.ServiceList{}
	if err := kubeClient.List(t.Context(), services); err != nil || len(services.Items) != 2 {
		t.Errorf("services = %d (err %v), want 2", len(services.Items), err)
	}

	actual := getCore(t, kubeClient, core.Name)
	assertCondition(t, actual, rhafv1alpha1.ConditionConfigValid, metav1.ConditionTrue, reasonValid)
	assertCondition(t, actual, rhafv1alpha1.ConditionCoreAvailable, metav1.ConditionFalse, reasonUnavailable)
	if !containsString(actual.Finalizers, finalizerName) {
		t.Errorf("finalizers = %v, want %q", actual.Finalizers, finalizerName)
	}
}

func TestReconcileCoreDoesNotUpdateExistingPVC(t *testing.T) {
	core := testCore()
	existing := newDataPVC(core, "test")
	existing.Labels = map[string]string{"kept": "true"}
	reconciler, kubeClient := newCoreReconciler(t, core, existing)

	reconcileCore(t, reconciler, core)

	pvc := &corev1.PersistentVolumeClaim{}
	exists(t, kubeClient, "rhaf-core-data", pvc)
	if pvc.Labels["kept"] != "true" {
		t.Errorf("existing pvc was modified: %v", pvc.Labels)
	}
}

func TestReconcileCoreReportsAvailability(t *testing.T) {
	core := testCore()
	reconciler, kubeClient := newCoreReconciler(t, core)
	reconcileCore(t, reconciler, core)

	deployment := &appsv1.Deployment{}
	exists(t, kubeClient, "rhaf-core", deployment)
	deployment.Status.ReadyReplicas = 1
	deployment.Status.AvailableReplicas = 1
	deployment.Status.Conditions = []appsv1.DeploymentCondition{{Type: appsv1.DeploymentAvailable, Status: corev1.ConditionTrue}}
	if err := kubeClient.Status().Update(t.Context(), deployment); err != nil {
		t.Fatalf("update deployment status: %v", err)
	}

	result, err := reconciler.Reconcile(t.Context(), requestFor(core))
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if result.RequeueAfter != 0 {
		t.Errorf("requeueAfter = %s, want none once available", result.RequeueAfter)
	}
	assertCondition(t, getCore(t, kubeClient, core.Name), rhafv1alpha1.ConditionCoreAvailable, metav1.ConditionTrue, reasonAvailable)
}

func TestReconcileCoreReportsStartupFailure(t *testing.T) {
	core := testCore()
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "rhaf-core-abc", Namespace: "test", Labels: coreSelectorLabels()},
		Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{{
			Name: coreContainerName,
			State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{
				Reason: "CrashLoopBackOff", Message: "back-off restarting failed container",
			}},
		}}},
	}
	reconciler, kubeClient := newCoreReconciler(t, core)
	reconcileCore(t, reconciler, core)
	if err := kubeClient.Create(t.Context(), pod); err != nil {
		t.Fatalf("create pod: %v", err)
	}

	result, err := reconciler.Reconcile(t.Context(), requestFor(core))
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if result.RequeueAfter != unavailableRequeue {
		t.Errorf("requeueAfter = %s, want %s", result.RequeueAfter, unavailableRequeue)
	}
	actual := getCore(t, kubeClient, core.Name)
	assertCondition(t, actual, rhafv1alpha1.ConditionCoreAvailable, metav1.ConditionFalse, "CrashLoopBackOff")
}

func TestReconcileCoreKeepsOnlyOneActiveInstallation(t *testing.T) {
	first := testCore()
	first.CreationTimestamp = metav1.NewTime(time.Now().Add(-time.Minute))
	second := testCore()
	second.Name = "another-core"
	second.UID = "second-uid"
	second.CreationTimestamp = metav1.Now()
	reconciler, kubeClient := newCoreReconciler(t, first, second)

	reconcileCore(t, reconciler, second)

	if exists(t, kubeClient, "rhaf-core", &appsv1.Deployment{}) {
		t.Fatal("duplicate installation created a deployment")
	}
	assertCondition(t, getCore(t, kubeClient, second.Name), rhafv1alpha1.ConditionConfigValid, metav1.ConditionFalse, reasonDuplicateInstallation)

	reconcileCore(t, reconciler, first)
	deployment := &appsv1.Deployment{}
	if !exists(t, kubeClient, "rhaf-core", deployment) {
		t.Fatal("singleton did not create a deployment")
	}

	// The active owner wins even if a contender would sort before it.
	second = getCore(t, kubeClient, second.Name)
	second.CreationTimestamp = first.CreationTimestamp
	if err := kubeClient.Update(t.Context(), second); err != nil {
		t.Fatalf("update contender: %v", err)
	}
	reconcileCore(t, reconciler, second)
	assertCondition(t, getCore(t, kubeClient, second.Name), rhafv1alpha1.ConditionConfigValid, metav1.ConditionFalse, reasonDuplicateInstallation)
	if !exists(t, kubeClient, "rhaf-core", deployment) {
		t.Fatal("active deployment disappeared")
	}
	if owner := metav1.GetControllerOf(deployment); owner == nil || owner.UID != first.UID {
		t.Errorf("deployment controller owner = %v, want %s", owner, first.Name)
	}

	if err := kubeClient.Delete(t.Context(), getCore(t, kubeClient, first.Name)); err != nil {
		t.Fatalf("delete first Core: %v", err)
	}
	reconcileCore(t, reconciler, first)
	requests := reconciler.remainingCores(t.Context(), first)
	if len(requests) != 1 || requests[0].NamespacedName != (client.ObjectKey{Name: second.Name}) {
		t.Fatalf("requests after deletion = %v, want %s", requests, second.Name)
	}
}

func TestReconcileCoreWanakuConfig(t *testing.T) {
	core := testCore()
	core.Spec.WanakuConfig = &rhafv1alpha1.RHAFPortalCoreWanakuConfigSpec{
		ConfigMapRef: rhafv1alpha1.ConfigMapKeyReference{Name: "wanaku", Key: "config.yaml"},
	}

	t.Run("missing configmap holds the deployment", func(t *testing.T) {
		reconciler, kubeClient := newCoreReconciler(t, core.DeepCopy())
		reconcileCore(t, reconciler, core)

		if exists(t, kubeClient, "rhaf-core", &appsv1.Deployment{}) {
			t.Fatal("deployment created without its ConfigMap")
		}
		actual := getCore(t, kubeClient, core.Name)
		assertCondition(t, actual, rhafv1alpha1.ConditionConfigValid, metav1.ConditionFalse, reasonConfigMapNotFound)
		assertCondition(t, actual, rhafv1alpha1.ConditionCoreAvailable, metav1.ConditionFalse, reasonNotDeployed)
	})

	t.Run("missing key holds the deployment", func(t *testing.T) {
		configMap := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "wanaku", Namespace: "test"}, Data: map[string]string{"other": "x"}}
		reconciler, kubeClient := newCoreReconciler(t, core.DeepCopy(), configMap)
		reconcileCore(t, reconciler, core)

		if exists(t, kubeClient, "rhaf-core", &appsv1.Deployment{}) {
			t.Fatal("deployment created without its ConfigMap key")
		}
		assertCondition(t, getCore(t, kubeClient, core.Name), rhafv1alpha1.ConditionConfigValid, metav1.ConditionFalse, reasonConfigMapKeyNotFound)
	})

	t.Run("content change rolls the pod template", func(t *testing.T) {
		configMap := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "wanaku", Namespace: "test"}, Data: map[string]string{"config.yaml": "a: 1"}}
		reconciler, kubeClient := newCoreReconciler(t, core.DeepCopy(), configMap)
		reconcileCore(t, reconciler, core)

		deployment := &appsv1.Deployment{}
		exists(t, kubeClient, "rhaf-core", deployment)
		before := deployment.Spec.Template.Annotations[wanakuConfigHashAnnotation]
		if before == "" {
			t.Fatal("config hash annotation missing")
		}

		configMap.Data["config.yaml"] = "a: 2"
		if err := kubeClient.Update(t.Context(), configMap); err != nil {
			t.Fatalf("update configmap: %v", err)
		}
		reconcileCore(t, reconciler, core)
		exists(t, kubeClient, "rhaf-core", deployment)
		if after := deployment.Spec.Template.Annotations[wanakuConfigHashAnnotation]; after == before {
			t.Fatalf("config hash unchanged after content change: %s", after)
		}
	})

	t.Run("configmap events map to referencing cores", func(t *testing.T) {
		reconciler, _ := newCoreReconciler(t, core.DeepCopy())
		requests := reconciler.coresForConfigMap(t.Context(), &metav1.PartialObjectMetadata{
			ObjectMeta: metav1.ObjectMeta{Name: "wanaku", Namespace: "test"},
		})
		if len(requests) != 1 || requests[0].NamespacedName != (client.ObjectKey{Name: core.Name}) {
			t.Fatalf("requests = %v, want only %s", requests, core.Name)
		}
		if requests := reconciler.coresForConfigMap(t.Context(), &metav1.PartialObjectMetadata{
			ObjectMeta: metav1.ObjectMeta{Name: "wanaku", Namespace: "other"},
		}); len(requests) != 0 {
			t.Fatalf("requests for another namespace = %v, want none", requests)
		}
	})
}

func TestReconcileCoreDeletionRetainsPVC(t *testing.T) {
	core := testCore()
	reconciler, kubeClient := newCoreReconciler(t, core)
	reconcileCore(t, reconciler, core)

	if err := kubeClient.Delete(t.Context(), getCore(t, kubeClient, core.Name)); err != nil {
		t.Fatalf("delete core: %v", err)
	}
	reconcileCore(t, reconciler, core)

	if err := kubeClient.Get(t.Context(), client.ObjectKey{Name: core.Name}, &rhafv1alpha1.RHAFPortalCore{}); !apierrors.IsNotFound(err) {
		t.Fatalf("get deleted core: %v, want not found", err)
	}
	if !exists(t, kubeClient, "rhaf-core-data", &corev1.PersistentVolumeClaim{}) {
		t.Fatal("pvc removed with the installation")
	}
}

func TestReconcileCoreWaitsForPreviousInstallationCleanup(t *testing.T) {
	core := testCore()
	// The Deployment is still controlled by a removed installation awaiting garbage collection.
	stale := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
		Name: "rhaf-core", Namespace: "test",
		OwnerReferences: []metav1.OwnerReference{{
			APIVersion: rhafv1alpha1.GroupVersion.String(), Kind: "RHAFPortalCore",
			Name: "removed", UID: "removed-uid", Controller: new(true),
		}},
	}}
	reconciler, kubeClient := newCoreReconciler(t, core, stale)

	result, err := reconciler.Reconcile(t.Context(), requestFor(core))
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if result.RequeueAfter != cleanupRequeue {
		t.Errorf("requeueAfter = %s, want %s", result.RequeueAfter, cleanupRequeue)
	}
	assertCondition(t, getCore(t, kubeClient, core.Name), rhafv1alpha1.ConditionCoreAvailable, metav1.ConditionFalse, reasonPreviousInstallationCleanup)
}

func TestReconcileCoreReportsApplyErrorsInStatus(t *testing.T) {
	core := testCore()
	kubeClient := fake.NewClientBuilder().
		WithScheme(coreTestScheme(t)).
		WithObjects(core).
		WithStatusSubresource(&rhafv1alpha1.RHAFPortalCore{}).
		WithInterceptorFuncs(interceptor.Funcs{Create: func(ctx context.Context, c client.WithWatch, object client.Object, opts ...client.CreateOption) error {
			if _, ok := object.(*appsv1.Deployment); ok {
				return apierrors.NewBadRequest("resources exceed limits")
			}

			return c.Create(ctx, object, opts...)
		}}).
		Build()
	reconciler := &RHAFPortalCoreReconciler{Client: kubeClient, APIReader: kubeClient, WorkloadNamespace: "test"}

	if _, err := reconciler.Reconcile(t.Context(), requestFor(core)); err == nil {
		t.Fatal("reconcile succeeded, want error")
	}
	assertCondition(t, getCore(t, kubeClient, core.Name), rhafv1alpha1.ConditionCoreAvailable, metav1.ConditionFalse, reasonReconcileError)
}

func TestReconcileCoreWaitsForPreviousCorePods(t *testing.T) {
	core := testCore()
	// The previous installation's Deployment is gone, but its pod still holds the data lock.
	terminating := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "rhaf-core-old", Namespace: "test", Labels: coreSelectorLabels()},
		Status:     corev1.PodStatus{Phase: corev1.PodRunning},
	}
	finished := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "rhaf-core-done", Namespace: "test", Labels: coreSelectorLabels()},
		Status:     corev1.PodStatus{Phase: corev1.PodFailed},
	}
	reconciler, kubeClient := newCoreReconciler(t, core, terminating, finished)

	result, err := reconciler.Reconcile(t.Context(), requestFor(core))
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if result.RequeueAfter != cleanupRequeue {
		t.Errorf("requeueAfter = %s, want %s", result.RequeueAfter, cleanupRequeue)
	}
	if exists(t, kubeClient, "rhaf-core", &appsv1.Deployment{}) {
		t.Fatal("deployment created while a previous Core pod is running")
	}
	assertCondition(t, getCore(t, kubeClient, core.Name), rhafv1alpha1.ConditionCoreAvailable, metav1.ConditionFalse, reasonPreviousInstallationCleanup)

	if err := kubeClient.Delete(t.Context(), terminating); err != nil {
		t.Fatalf("delete previous pod: %v", err)
	}
	reconcileCore(t, reconciler, core)
	if !exists(t, kubeClient, "rhaf-core", &appsv1.Deployment{}) {
		t.Fatal("deployment not created after the previous pod exited")
	}
}
