// Copyright 2026 The RHAF Portal Operator Authors
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	rhafv1alpha1 "github.com/rhaf-ai/rhaf-portal-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Condition reasons for RHAFPortalCore.
const (
	reasonValid                 = "Valid"
	reasonDuplicateInstallation = "DuplicateInstallation"
	reasonConfigMapNotFound     = "ConfigMapNotFound"
	reasonConfigMapKeyNotFound  = "ConfigMapKeyNotFound"
	reasonAvailable             = "Available"
	reasonUnavailable           = "Unavailable"
	reasonNotDeployed           = "NotDeployed"
	reasonProgressDeadline      = "ProgressDeadlineExceeded"
	reasonReconcileError        = "ReconcileError"

	reasonPreviousInstallationCleanup = "PreviousInstallationCleanup"
)

const (
	// wanakuConfigMapIndex indexes RHAFPortalCore objects by referenced ConfigMap name.
	wanakuConfigMapIndex = "spec.wanakuConfig.configMapRef.name"
	// unavailableRequeue polls pod state while Core is not available, because pod
	// failures such as CrashLoopBackOff do not change the Deployment status.
	unavailableRequeue = 30 * time.Second
	// cleanupRequeue waits for garbage collection of a removed installation's resources.
	cleanupRequeue = 5 * time.Second
)

// RHAFPortalCoreReconciler reconciles a RHAFPortalCore resource.
type RHAFPortalCoreReconciler struct {
	client.Client

	// APIReader reads objects that the manager does not cache: ConfigMaps (watched
	// as metadata only) and Pods.
	APIReader client.Reader
}

// +kubebuilder:rbac:groups=rhaf.ai.redhat.com,resources=rhafportalcores,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rhaf.ai.redhat.com,resources=rhafportalcores/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=rhaf.ai.redhat.com,resources=rhafportalcores/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list

// Reconcile installs the Core Deployment, its data PVC, and the internal Services.
func (r *RHAFPortalCoreReconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	core := &rhafv1alpha1.RHAFPortalCore{}
	if err := r.Get(ctx, request.NamespacedName, core); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Deployment and Services are garbage-collected through owner references.
	// The PVC has no owner reference and is retained.
	if err := reconcileFinalizer(ctx, r.Client, core); err != nil || !core.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, err
	}

	original := core.DeepCopy()
	result, reconcileErr := r.reconcileWorkload(ctx, core)
	var alreadyOwned *controllerutil.AlreadyOwnedError
	switch {
	case errors.As(reconcileErr, &alreadyOwned):
		// A removed installation still controls the resource until garbage collection deletes it.
		setCondition(core, rhafv1alpha1.ConditionCoreAvailable, metav1.ConditionFalse, reasonPreviousInstallationCleanup,
			fmt.Sprintf("Waiting for the removal of resources owned by RHAFPortalCore %q", alreadyOwned.Owner.Name))
		result, reconcileErr = ctrl.Result{RequeueAfter: cleanupRequeue}, nil
	case reconcileErr != nil:
		setCondition(core, rhafv1alpha1.ConditionCoreAvailable, metav1.ConditionFalse, reasonReconcileError, reconcileErr.Error())
	}

	core.Status.ObservedGeneration = core.Generation
	if err := r.Status().Patch(ctx, core, client.MergeFrom(original)); err != nil {
		return ctrl.Result{}, errors.Join(reconcileErr, fmt.Errorf("update status: %w", err))
	}

	return result, reconcileErr
}

func (r *RHAFPortalCoreReconciler) reconcileWorkload(ctx context.Context, core *rhafv1alpha1.RHAFPortalCore) (ctrl.Result, error) {
	active, err := r.activeInstallation(ctx, core.Namespace)
	if err != nil {
		return ctrl.Result{}, err
	}
	if active != core.Name {
		setCondition(core, rhafv1alpha1.ConditionConfigValid, metav1.ConditionFalse, reasonDuplicateInstallation,
			fmt.Sprintf("RHAFPortalCore %q already manages this namespace", active))
		setCondition(core, rhafv1alpha1.ConditionCoreAvailable, metav1.ConditionFalse, reasonNotDeployed,
			"This installation is not active")

		return ctrl.Result{}, nil
	}

	if err := r.ensureDataPVC(ctx, core); err != nil {
		return ctrl.Result{}, err
	}
	if err := r.applyService(ctx, core, coreManagementServiceName, managementPortName, managementPort(core)); err != nil {
		return ctrl.Result{}, err
	}
	if err := r.applyService(ctx, core, coreMCPServiceName, mcpPortName, mcpPort(core)); err != nil {
		return ctrl.Result{}, err
	}

	configHash, reason, message, err := r.resolveWanakuConfig(ctx, core)
	if err != nil {
		return ctrl.Result{}, err
	}
	if reason != "" {
		setCondition(core, rhafv1alpha1.ConditionConfigValid, metav1.ConditionFalse, reason, message)

		return r.reportAvailability(ctx, core)
	}
	setCondition(core, rhafv1alpha1.ConditionConfigValid, metav1.ConditionTrue, reasonValid, "Configuration is valid")

	deployment := &appsv1.Deployment{Name: coreDeploymentName, Namespace: core.Namespace}
	waiting, err := r.previousCoreRunning(ctx, deployment)
	if err != nil {
		return ctrl.Result{}, err
	}
	if waiting {
		setCondition(core, rhafv1alpha1.ConditionCoreAvailable, metav1.ConditionFalse, reasonPreviousInstallationCleanup,
			"Waiting for the previous Core pod to exit and release the data volume")

		return ctrl.Result{RequeueAfter: cleanupRequeue}, nil
	}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, deployment, func() error {
		mutateDeployment(deployment, core, configHash)

		return controllerutil.SetControllerReference(core, deployment, r.Scheme())
	}); err != nil {
		return ctrl.Result{}, fmt.Errorf("apply deployment: %w", err)
	}

	return r.reportAvailability(ctx, core)
}

// previousCoreRunning reports whether the Core Deployment must not be created yet
// because a Core pod from a removed installation still runs. That pod holds the
// exclusive data-directory lock until it exits, and a new pod would fail to start.
// The check applies only when the Deployment does not exist: Recreate already
// orders pods within one Deployment.
func (r *RHAFPortalCoreReconciler) previousCoreRunning(ctx context.Context, deployment *appsv1.Deployment) (bool, error) {
	err := r.Get(ctx, client.ObjectKeyFromObject(deployment), &appsv1.Deployment{})
	if !apierrors.IsNotFound(err) {
		return false, err
	}

	pods := &corev1.PodList{}
	if err := r.APIReader.List(ctx, pods, client.InNamespace(deployment.Namespace), client.MatchingLabels(coreSelectorLabels())); err != nil {
		return false, fmt.Errorf("list Core pods: %w", err)
	}

	return slices.ContainsFunc(pods.Items, func(pod corev1.Pod) bool {
		return pod.Status.Phase != corev1.PodSucceeded && pod.Status.Phase != corev1.PodFailed
	}), nil
}

// activeInstallation returns the name of the RHAFPortalCore that manages the
// namespace. An installation that controls the Core Deployment or one of the Core
// Services stays active. Otherwise the oldest object is active, with the name as a
// tie-breaker.
func (r *RHAFPortalCoreReconciler) activeInstallation(ctx context.Context, namespace string) (string, error) {
	cores := &rhafv1alpha1.RHAFPortalCoreList{}
	if err := r.List(ctx, cores, client.InNamespace(namespace)); err != nil {
		return "", fmt.Errorf("list installations: %w", err)
	}
	if len(cores.Items) == 0 {
		return "", nil
	}

	managed := []client.Object{
		&appsv1.Deployment{Name: coreDeploymentName, Namespace: namespace},
		&corev1.Service{Name: coreManagementServiceName, Namespace: namespace},
		&corev1.Service{Name: coreMCPServiceName, Namespace: namespace},
	}
	for _, object := range managed {
		err := r.Get(ctx, client.ObjectKeyFromObject(object), object)
		if apierrors.IsNotFound(err) {
			continue
		}
		if err != nil {
			return "", fmt.Errorf("get %s: %w", object.GetName(), err)
		}
		if owner := metav1.GetControllerOf(object); owner != nil {
			for _, core := range cores.Items {
				if core.UID == owner.UID {
					return core.Name, nil
				}
			}
		}
	}

	oldest := slices.MinFunc(cores.Items, func(left, right rhafv1alpha1.RHAFPortalCore) int {
		if order := left.CreationTimestamp.Compare(right.CreationTimestamp.Time); order != 0 {
			return order
		}

		return strings.Compare(left.Name, right.Name)
	})

	return oldest.Name, nil
}

// ensureDataPVC creates the data claim once. Size and storage class are not
// updated afterwards because most of the claim specification is immutable.
func (r *RHAFPortalCoreReconciler) ensureDataPVC(ctx context.Context, core *rhafv1alpha1.RHAFPortalCore) error {
	err := r.Get(ctx, types.NamespacedName{Name: coreDataPVCName, Namespace: core.Namespace}, &corev1.PersistentVolumeClaim{})
	if !apierrors.IsNotFound(err) {
		return err
	}
	if err := r.Create(ctx, newDataPVC(core)); err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("create data PVC: %w", err)
	}

	return nil
}

func (r *RHAFPortalCoreReconciler) applyService(ctx context.Context, core *rhafv1alpha1.RHAFPortalCore, name, portName string, port int32) error {
	service := &corev1.Service{Name: name, Namespace: core.Namespace}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, service, func() error {
		mutateService(service, core, portName, port)

		return controllerutil.SetControllerReference(core, service, r.Scheme())
	}); err != nil {
		return fmt.Errorf("apply service %s: %w", name, err)
	}

	return nil
}

// resolveWanakuConfig returns the content hash of the referenced ConfigMap key, or
// a condition reason and message when the reference cannot be resolved.
func (r *RHAFPortalCoreReconciler) resolveWanakuConfig(ctx context.Context, core *rhafv1alpha1.RHAFPortalCore) (string, string, string, error) {
	if core.Spec.WanakuConfig == nil {
		return "", "", "", nil
	}

	reference := core.Spec.WanakuConfig.ConfigMapRef
	configMap := &corev1.ConfigMap{}
	err := r.APIReader.Get(ctx, types.NamespacedName{Name: reference.Name, Namespace: core.Namespace}, configMap)
	if apierrors.IsNotFound(err) {
		return "", reasonConfigMapNotFound, fmt.Sprintf("ConfigMap %q does not exist", reference.Name), nil
	}
	if err != nil {
		return "", "", "", fmt.Errorf("get ConfigMap %s: %w", reference.Name, err)
	}

	content, ok := configMap.Data[reference.Key]
	if !ok {
		return "", reasonConfigMapKeyNotFound, fmt.Sprintf("ConfigMap %q has no key %q", reference.Name, reference.Key), nil
	}

	return wanakuConfigHash(content), "", "", nil
}

// reportAvailability sets CoreAvailable from the Deployment and Core pod state.
func (r *RHAFPortalCoreReconciler) reportAvailability(ctx context.Context, core *rhafv1alpha1.RHAFPortalCore) (ctrl.Result, error) {
	deployment := &appsv1.Deployment{}
	err := r.Get(ctx, types.NamespacedName{Name: coreDeploymentName, Namespace: core.Namespace}, deployment)
	if apierrors.IsNotFound(err) {
		setCondition(core, rhafv1alpha1.ConditionCoreAvailable, metav1.ConditionFalse, reasonNotDeployed, "The Core Deployment does not exist")

		return ctrl.Result{}, nil
	}
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("get deployment: %w", err)
	}

	if deploymentAvailable(deployment) {
		setCondition(core, rhafv1alpha1.ConditionCoreAvailable, metav1.ConditionTrue, reasonAvailable, "Core is ready")

		return ctrl.Result{}, nil
	}

	reason, message, err := r.podFailure(ctx, core)
	if err != nil {
		return ctrl.Result{}, err
	}
	if reason == "" {
		reason, message = reasonUnavailable, "Waiting for the Core pod to become ready"
		for _, condition := range deployment.Status.Conditions {
			if condition.Type == appsv1.DeploymentProgressing && condition.Reason == reasonProgressDeadline {
				reason, message = reasonProgressDeadline, condition.Message
			}
		}
	}
	setCondition(core, rhafv1alpha1.ConditionCoreAvailable, metav1.ConditionFalse, reason, message)

	return ctrl.Result{RequeueAfter: unavailableRequeue}, nil
}

func deploymentAvailable(deployment *appsv1.Deployment) bool {
	if deployment.Status.ObservedGeneration < deployment.Generation || deployment.Status.ReadyReplicas < 1 {
		return false
	}
	for _, condition := range deployment.Status.Conditions {
		if condition.Type == appsv1.DeploymentAvailable {
			return condition.Status == corev1.ConditionTrue
		}
	}

	return false
}

// podFailure returns the waiting or termination reason of a failing Core container.
func (r *RHAFPortalCoreReconciler) podFailure(ctx context.Context, core *rhafv1alpha1.RHAFPortalCore) (string, string, error) {
	pods := &corev1.PodList{}
	if err := r.APIReader.List(ctx, pods, client.InNamespace(core.Namespace), client.MatchingLabels(coreSelectorLabels())); err != nil {
		return "", "", fmt.Errorf("list Core pods: %w", err)
	}

	for _, pod := range pods.Items {
		for _, status := range pod.Status.ContainerStatuses {
			if status.Name != coreContainerName {
				continue
			}
			if waiting := status.State.Waiting; waiting != nil && waiting.Reason != "" && waiting.Reason != "ContainerCreating" {
				return waiting.Reason, waiting.Message, nil
			}
			if terminated := status.LastTerminationState.Terminated; terminated != nil && !status.Ready {
				return terminated.Reason, fmt.Sprintf("Core exited with code %d: %s", terminated.ExitCode, terminated.Message), nil
			}
		}
	}

	return "", "", nil
}

func setCondition(core *rhafv1alpha1.RHAFPortalCore, conditionType string, status metav1.ConditionStatus, reason, message string) {
	meta.SetStatusCondition(&core.Status.Conditions, metav1.Condition{
		Type:               conditionType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: core.Generation,
	})
}

func indexWanakuConfigMap(object client.Object) []string {
	core, ok := object.(*rhafv1alpha1.RHAFPortalCore)
	if !ok || core.Spec.WanakuConfig == nil {
		return nil
	}

	return []string{core.Spec.WanakuConfig.ConfigMapRef.Name}
}

// coresForConfigMap enqueues the installations that reference a ConfigMap.
func (r *RHAFPortalCoreReconciler) coresForConfigMap(ctx context.Context, configMap client.Object) []reconcile.Request {
	cores := &rhafv1alpha1.RHAFPortalCoreList{}
	if err := r.List(ctx, cores, client.InNamespace(configMap.GetNamespace()),
		client.MatchingFields{wanakuConfigMapIndex: configMap.GetName()}); err != nil {
		ctrl.LoggerFrom(ctx).Error(err, "list installations for ConfigMap", "configMap", configMap.GetName())

		return nil
	}

	return requestsFor(cores.Items, "")
}

// siblingCores enqueues the other installations in a namespace, so a waiting
// duplicate becomes active when the active installation is removed.
func (r *RHAFPortalCoreReconciler) siblingCores(ctx context.Context, object client.Object) []reconcile.Request {
	cores := &rhafv1alpha1.RHAFPortalCoreList{}
	if err := r.List(ctx, cores, client.InNamespace(object.GetNamespace())); err != nil {
		ctrl.LoggerFrom(ctx).Error(err, "list sibling installations")

		return nil
	}

	return requestsFor(cores.Items, object.GetName())
}

func requestsFor(cores []rhafv1alpha1.RHAFPortalCore, exclude string) []reconcile.Request {
	requests := make([]reconcile.Request, 0, len(cores))
	for _, core := range cores {
		if core.Name != exclude {
			requests = append(requests, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(&core)})
		}
	}

	return requests
}

// SetupWithManager registers the controller with the manager.
func (r *RHAFPortalCoreReconciler) SetupWithManager(manager ctrl.Manager) error {
	if err := manager.GetFieldIndexer().IndexField(context.Background(), &rhafv1alpha1.RHAFPortalCore{},
		wanakuConfigMapIndex, indexWanakuConfigMap); err != nil {
		return fmt.Errorf("index installations by ConfigMap: %w", err)
	}

	return ctrl.NewControllerManagedBy(manager).
		For(&rhafv1alpha1.RHAFPortalCore{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Watches(&corev1.ConfigMap{}, handler.EnqueueRequestsFromMapFunc(r.coresForConfigMap), builder.OnlyMetadata).
		Watches(&rhafv1alpha1.RHAFPortalCore{}, handler.EnqueueRequestsFromMapFunc(r.siblingCores)).
		Complete(r)
}
