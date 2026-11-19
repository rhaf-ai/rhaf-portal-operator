// Copyright 2026 The RHAF Portal Operator Authors
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"maps"

	rhafv1alpha1 "github.com/rhaf-ai/rhaf-portal-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// Names of the operator-owned Core resources. One Core installation exists per namespace.
const (
	coreDeploymentName        = "rhaf-core"
	coreDataPVCName           = "rhaf-core-data"
	coreMCPServiceName        = "rhaf-core-mcp"
	coreManagementServiceName = "rhaf-core-management"
)

const (
	coreContainerName          = "core"
	managementPortName         = "management"
	mcpPortName                = "mcp"
	dataVolumeName             = "data"
	wanakuConfigVolumeName     = "wanaku-config"
	tmpVolumeName              = "tmp"
	tmpPath                    = "/tmp"
	coreDataPath               = "/var/lib/rhaf-portal"
	wanakuConfigDir            = "/etc/rhaf-portal/config"
	wanakuConfigFile           = "wanaku.yaml"
	wanakuConfigHashAnnotation = "rhaf.ai.redhat.com/wanaku-config-hash"
	healthPath                 = "/healthz"
	configFileMode             = int32(0o444)
)

// Probe settings. Every field is explicit so API server defaults do not cause an
// update on every reconcile.
const (
	probePeriodSeconds        = 10
	probeTimeoutSeconds       = 1
	livenessFailureThreshold  = 3
	readinessFailureThreshold = 1
)

// coreSelectorLabels returns the immutable labels that select the Core pod. They
// omit the installation name: one Core runs per namespace, and a Deployment adopted
// by a differently named installation must keep a selector that matches its template.
func coreSelectorLabels() map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":      "rhaf-portal-core",
		"app.kubernetes.io/component": "core",
	}
}

func coreLabels(core *rhafv1alpha1.RHAFPortalCore) map[string]string {
	labels := coreSelectorLabels()
	labels["app.kubernetes.io/instance"] = core.Name
	labels["app.kubernetes.io/managed-by"] = "rhaf-portal-operator"

	return labels
}

func managementPort(core *rhafv1alpha1.RHAFPortalCore) int32 {
	if core.Spec.Management.Port != 0 {
		return core.Spec.Management.Port
	}

	return rhafv1alpha1.DefaultManagementPort
}

func mcpPort(core *rhafv1alpha1.RHAFPortalCore) int32 {
	if core.Spec.MCP.Port != 0 {
		return core.Spec.MCP.Port
	}

	return rhafv1alpha1.DefaultMCPPort
}

func coreArgs(core *rhafv1alpha1.RHAFPortalCore) []string {
	args := []string{
		fmt.Sprintf("--management-listen=0.0.0.0:%d", managementPort(core)),
		fmt.Sprintf("--mcp-listen=0.0.0.0:%d", mcpPort(core)),
		"--data-dir=" + coreDataPath,
	}
	if core.Spec.WanakuConfig != nil {
		args = append(args, "--wanaku-config="+wanakuConfigDir+"/"+wanakuConfigFile)
	}

	return args
}

// wanakuConfigHash returns a stable digest of the bootstrap configuration content.
func wanakuConfigHash(content string) string {
	sum := sha256.Sum256([]byte(content))

	return hex.EncodeToString(sum[:])
}

// mutateDeployment sets the operator-managed fields of the Core Deployment in place.
// Fields defaulted by the API server are preserved so repeated reconciles do not update.
func mutateDeployment(deployment *appsv1.Deployment, core *rhafv1alpha1.RHAFPortalCore, configHash string) {
	if deployment.Labels == nil {
		deployment.Labels = map[string]string{}
	}
	maps.Copy(deployment.Labels, coreLabels(core))

	if deployment.Spec.Selector == nil {
		deployment.Spec.Selector = &metav1.LabelSelector{MatchLabels: coreSelectorLabels()}
	}
	deployment.Spec.Replicas = new(int32(1))
	// Core holds an exclusive lock on the data directory, so pods must not overlap.
	deployment.Spec.Strategy = appsv1.DeploymentStrategy{Type: appsv1.RecreateDeploymentStrategyType}

	template := &deployment.Spec.Template
	if template.Labels == nil {
		template.Labels = map[string]string{}
	}
	maps.Copy(template.Labels, coreLabels(core))
	if core.Spec.WanakuConfig != nil {
		if template.Annotations == nil {
			template.Annotations = map[string]string{}
		}
		template.Annotations[wanakuConfigHashAnnotation] = configHash
	} else {
		delete(template.Annotations, wanakuConfigHashAnnotation)
	}

	template.Spec.Volumes = coreVolumes(core)
	// Core does not call the Kubernetes API.
	template.Spec.AutomountServiceAccountToken = new(false)
	template.Spec.SecurityContext = podSecurityContext(core)

	index := containerIndex(template.Spec.Containers, coreContainerName)
	if index < 0 {
		template.Spec.Containers = append(template.Spec.Containers, corev1.Container{Name: coreContainerName})
		index = len(template.Spec.Containers) - 1
	}
	mutateCoreContainer(&template.Spec.Containers[index], core)
}

func containerIndex(containers []corev1.Container, name string) int {
	for index := range containers {
		if containers[index].Name == name {
			return index
		}
	}

	return -1
}

// podSecurityContext copies the user settings and enforces a non-root user and a
// RuntimeDefault seccomp profile when none is set.
func podSecurityContext(core *rhafv1alpha1.RHAFPortalCore) *corev1.PodSecurityContext {
	security := &corev1.PodSecurityContext{}
	if core.Spec.PodSecurityContext != nil {
		security = core.Spec.PodSecurityContext.DeepCopy()
	}
	security.RunAsNonRoot = new(true)
	if security.SeccompProfile == nil {
		security.SeccompProfile = &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault}
	}

	return security
}

func coreVolumes(core *rhafv1alpha1.RHAFPortalCore) []corev1.Volume {
	volumes := []corev1.Volume{
		{
			Name:                  dataVolumeName,
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: coreDataPVCName},
		},
		// The root filesystem is read-only; /tmp stays writable for runtime scratch files.
		{Name: tmpVolumeName, EmptyDir: &corev1.EmptyDirVolumeSource{}},
	}
	if core.Spec.WanakuConfig != nil {
		reference := core.Spec.WanakuConfig.ConfigMapRef
		volumes = append(volumes, corev1.Volume{
			Name: wanakuConfigVolumeName,
			ConfigMap: &corev1.ConfigMapVolumeSource{
				Name:        reference.Name,
				Items:       []corev1.KeyToPath{{Key: reference.Key, Path: wanakuConfigFile}},
				DefaultMode: new(configFileMode),
			},
		})
	}

	return volumes
}

func mutateCoreContainer(container *corev1.Container, core *rhafv1alpha1.RHAFPortalCore) {
	container.Image = core.Spec.Image
	container.Args = coreArgs(core)
	container.Ports = []corev1.ContainerPort{
		{Name: managementPortName, ContainerPort: managementPort(core), Protocol: corev1.ProtocolTCP},
		{Name: mcpPortName, ContainerPort: mcpPort(core), Protocol: corev1.ProtocolTCP},
	}
	container.Resources = core.Spec.Resources

	container.VolumeMounts = []corev1.VolumeMount{
		{Name: dataVolumeName, MountPath: coreDataPath},
		{Name: tmpVolumeName, MountPath: tmpPath},
	}
	if core.Spec.WanakuConfig != nil {
		container.VolumeMounts = append(container.VolumeMounts,
			corev1.VolumeMount{Name: wanakuConfigVolumeName, MountPath: wanakuConfigDir, ReadOnly: true})
	}

	container.LivenessProbe = healthProbe(livenessFailureThreshold)
	container.ReadinessProbe = healthProbe(readinessFailureThreshold)
	container.SecurityContext = &corev1.SecurityContext{
		RunAsNonRoot:             new(true),
		AllowPrivilegeEscalation: new(false),
		ReadOnlyRootFilesystem:   new(true),
		Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
		// The seccomp profile is inherited from the pod security context.
	}
}

// healthProbe checks the management /healthz route.
func healthProbe(failureThreshold int32) *corev1.Probe {
	return &corev1.Probe{
		HTTPGet: &corev1.HTTPGetAction{
			Path:   healthPath,
			Port:   intstr.FromString(managementPortName),
			Scheme: corev1.URISchemeHTTP,
		},
		PeriodSeconds:    probePeriodSeconds,
		TimeoutSeconds:   probeTimeoutSeconds,
		SuccessThreshold: 1,
		FailureThreshold: failureThreshold,
	}
}

// mutateService sets the operator-managed fields of an internal Core Service in place.
func mutateService(service *corev1.Service, core *rhafv1alpha1.RHAFPortalCore, portName string, port int32) {
	if service.Labels == nil {
		service.Labels = map[string]string{}
	}
	maps.Copy(service.Labels, coreLabels(core))
	service.Spec.Type = corev1.ServiceTypeClusterIP
	service.Spec.Selector = coreSelectorLabels()
	service.Spec.Ports = []corev1.ServicePort{{
		Name:       portName,
		Protocol:   corev1.ProtocolTCP,
		Port:       port,
		TargetPort: intstr.FromString(portName),
	}}
}

// newDataPVC returns the Core data claim. It carries no owner reference so that
// deleting the installation retains the data.
func newDataPVC(core *rhafv1alpha1.RHAFPortalCore) *corev1.PersistentVolumeClaim {
	size := core.Spec.Storage.Size
	if size.IsZero() {
		size = resource.MustParse(rhafv1alpha1.DefaultStorageSize)
	}

	pvc := &corev1.PersistentVolumeClaim{
		Name: coreDataPVCName, Namespace: core.Namespace, Labels: coreLabels(core),
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceStorage: size},
			},
		},
	}
	if core.Spec.Storage.StorageClassName != "" {
		pvc.Spec.StorageClassName = new(core.Spec.Storage.StorageClassName)
	}

	return pvc
}
