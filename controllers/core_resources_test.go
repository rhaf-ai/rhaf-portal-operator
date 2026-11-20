// Copyright 2026 The RHAF Portal Operator Authors
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"maps"
	"slices"
	"strings"
	"testing"

	rhafv1alpha1 "github.com/rhaf-ai/rhaf-portal-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/validation"
)

func testCore() *rhafv1alpha1.RHAFPortalCore {
	return &rhafv1alpha1.RHAFPortalCore{
		ObjectMeta: metav1.ObjectMeta{Name: "example.core", UID: "core-uid"},
		Spec:       rhafv1alpha1.RHAFPortalCoreSpec{Image: "quay.io/rhaf-ai/rhaf-portal-core:1.0"},
	}
}

func TestCoreLabelsAcceptLongResourceNames(t *testing.T) {
	core := testCore()
	core.Name = strings.Repeat(strings.Repeat("a", 63)+".", 3) + strings.Repeat("b", 61)
	label := coreLabels(core)["app.kubernetes.io/instance"]
	if errs := validation.IsValidLabelValue(label); len(errs) != 0 {
		t.Fatalf("instance label %q is invalid: %v", label, errs)
	}
	core.Name = core.Name[:len(core.Name)-1] + "c"
	if other := coreLabels(core)["app.kubernetes.io/instance"]; other == label {
		t.Errorf("distinct names produced the same instance label %q", label)
	}
}

func coreContainer(t *testing.T, deployment *appsv1.Deployment) corev1.Container {
	t.Helper()
	for _, container := range deployment.Spec.Template.Spec.Containers {
		if container.Name == coreContainerName {
			return container
		}
	}
	t.Fatalf("deployment has no %q container", coreContainerName)

	return corev1.Container{}
}

func TestMutateDeploymentDefaults(t *testing.T) {
	deployment := &appsv1.Deployment{}
	mutateDeployment(deployment, testCore(), "")

	if deployment.Spec.Replicas == nil || *deployment.Spec.Replicas != 1 {
		t.Errorf("replicas = %v, want 1", deployment.Spec.Replicas)
	}
	if deployment.Spec.Strategy.Type != appsv1.RecreateDeploymentStrategyType || deployment.Spec.Strategy.RollingUpdate != nil {
		t.Errorf("strategy = %+v, want Recreate", deployment.Spec.Strategy)
	}
	wantSelector := map[string]string{"app.kubernetes.io/name": "rhaf-portal-core", "app.kubernetes.io/component": "core"}
	if deployment.Spec.Selector == nil || !maps.Equal(deployment.Spec.Selector.MatchLabels, wantSelector) {
		t.Errorf("selector = %v, want %v", deployment.Spec.Selector, wantSelector)
	}

	container := coreContainer(t, deployment)
	if container.Image != "quay.io/rhaf-ai/rhaf-portal-core:1.0" {
		t.Errorf("image = %q", container.Image)
	}
	wantArgs := []string{"--management-listen=0.0.0.0:8080", "--mcp-listen=0.0.0.0:8081", "--data-dir=/var/lib/rhaf-portal"}
	if !slices.Equal(container.Args, wantArgs) {
		t.Errorf("args = %v, want %v", container.Args, wantArgs)
	}
	for _, probe := range []*corev1.Probe{container.LivenessProbe, container.ReadinessProbe} {
		if probe == nil || probe.HTTPGet == nil || probe.HTTPGet.Path != "/healthz" || probe.HTTPGet.Port != intstr.FromString("management") {
			t.Errorf("probe = %+v, want GET /healthz on port management", probe)
		}
	}
	if !slices.ContainsFunc(container.VolumeMounts, func(mount corev1.VolumeMount) bool {
		return mount.Name == dataVolumeName && mount.MountPath == "/var/lib/rhaf-portal"
	}) {
		t.Errorf("volume mounts = %v, want data mount", container.VolumeMounts)
	}
	volumes := deployment.Spec.Template.Spec.Volumes
	if len(volumes) != 2 || volumes[0].PersistentVolumeClaim == nil || volumes[0].PersistentVolumeClaim.ClaimName != "rhaf-core-data" ||
		volumes[1].EmptyDir == nil {
		t.Errorf("volumes = %+v, want the rhaf-core-data PVC and a tmp emptyDir", volumes)
	}
	if _, ok := deployment.Spec.Template.Annotations[wanakuConfigHashAnnotation]; ok {
		t.Errorf("unexpected config hash annotation without wanakuConfig")
	}
	security := container.SecurityContext
	if security == nil || security.RunAsNonRoot == nil || !*security.RunAsNonRoot ||
		security.AllowPrivilegeEscalation == nil || *security.AllowPrivilegeEscalation ||
		security.ReadOnlyRootFilesystem == nil || !*security.ReadOnlyRootFilesystem ||
		security.Capabilities == nil || !slices.Equal(security.Capabilities.Drop, []corev1.Capability{"ALL"}) {
		t.Errorf("security context = %+v, want restricted non-root container", security)
	}
	if !slices.ContainsFunc(container.VolumeMounts, func(mount corev1.VolumeMount) bool {
		return mount.Name == tmpVolumeName && mount.MountPath == "/tmp"
	}) {
		t.Errorf("volume mounts = %v, want /tmp mount", container.VolumeMounts)
	}
	podSpec := deployment.Spec.Template.Spec
	if podSpec.AutomountServiceAccountToken == nil || *podSpec.AutomountServiceAccountToken {
		t.Errorf("automountServiceAccountToken = %v, want false", podSpec.AutomountServiceAccountToken)
	}
}

func TestMutateDeploymentCustomPortsAndWanakuConfig(t *testing.T) {
	core := testCore()
	core.Spec.Management.Port = 9090
	core.Spec.MCP.Port = 9091
	core.Spec.WanakuConfig = &rhafv1alpha1.RHAFPortalCoreWanakuConfigSpec{
		ConfigMapRef: rhafv1alpha1.ConfigMapKeyReference{Name: "wanaku", Key: "bootstrap.yaml"},
	}
	deployment := &appsv1.Deployment{}
	mutateDeployment(deployment, core, "hash-1")

	container := coreContainer(t, deployment)
	wantArgs := []string{
		"--management-listen=0.0.0.0:9090", "--mcp-listen=0.0.0.0:9091", "--data-dir=/var/lib/rhaf-portal",
		"--wanaku-config=/etc/rhaf-portal/config/wanaku.yaml",
	}
	if !slices.Equal(container.Args, wantArgs) {
		t.Errorf("args = %v, want %v", container.Args, wantArgs)
	}
	if !slices.ContainsFunc(container.Ports, func(port corev1.ContainerPort) bool {
		return port.Name == "management" && port.ContainerPort == 9090
	}) || !slices.ContainsFunc(container.Ports, func(port corev1.ContainerPort) bool {
		return port.Name == "mcp" && port.ContainerPort == 9091
	}) {
		t.Errorf("ports = %v, want management 9090 and mcp 9091", container.Ports)
	}
	if deployment.Spec.Template.Annotations[wanakuConfigHashAnnotation] != "hash-1" {
		t.Errorf("annotations = %v, want hash-1", deployment.Spec.Template.Annotations)
	}

	var configVolume *corev1.Volume
	for index := range deployment.Spec.Template.Spec.Volumes {
		if deployment.Spec.Template.Spec.Volumes[index].Name == wanakuConfigVolumeName {
			configVolume = &deployment.Spec.Template.Spec.Volumes[index]
		}
	}
	if configVolume == nil || configVolume.ConfigMap == nil || configVolume.ConfigMap.Name != "wanaku" ||
		len(configVolume.ConfigMap.Items) != 1 || configVolume.ConfigMap.Items[0] != (corev1.KeyToPath{Key: "bootstrap.yaml", Path: "wanaku.yaml"}) {
		t.Errorf("config volume = %+v, want wanaku/bootstrap.yaml as wanaku.yaml", configVolume)
	}
	if !slices.ContainsFunc(container.VolumeMounts, func(mount corev1.VolumeMount) bool {
		return mount.Name == wanakuConfigVolumeName && mount.MountPath == "/etc/rhaf-portal/config" && mount.ReadOnly
	}) {
		t.Errorf("volume mounts = %v, want read-only config mount", container.VolumeMounts)
	}

	// Removing the configuration removes the volume, mount, argument, and annotation.
	core.Spec.WanakuConfig = nil
	mutateDeployment(deployment, core, "")
	container = coreContainer(t, deployment)
	if len(container.Args) != 3 || len(container.VolumeMounts) != 2 || len(deployment.Spec.Template.Spec.Volumes) != 2 {
		t.Errorf("config not removed: args=%v mounts=%v volumes=%v", container.Args, container.VolumeMounts, deployment.Spec.Template.Spec.Volumes)
	}
	if _, ok := deployment.Spec.Template.Annotations[wanakuConfigHashAnnotation]; ok {
		t.Errorf("config hash annotation not removed")
	}
}

func TestMutateDeploymentPreservesServerDefaults(t *testing.T) {
	deployment := &appsv1.Deployment{}
	mutateDeployment(deployment, testCore(), "")
	deployment.Spec.Template.Spec.Containers[0].TerminationMessagePath = "/dev/termination-log"
	deployment.Spec.Template.Spec.Containers[0].ImagePullPolicy = corev1.PullIfNotPresent

	mutateDeployment(deployment, testCore(), "")

	container := coreContainer(t, deployment)
	if container.TerminationMessagePath != "/dev/termination-log" || container.ImagePullPolicy != corev1.PullIfNotPresent {
		t.Errorf("server-defaulted fields overwritten: %+v", container)
	}
	if len(deployment.Spec.Template.Spec.Containers) != 1 {
		t.Errorf("containers = %d, want 1", len(deployment.Spec.Template.Spec.Containers))
	}
}

func TestMutateService(t *testing.T) {
	core := testCore()
	core.Spec.MCP.Port = 9191
	service := &corev1.Service{Spec: corev1.ServiceSpec{ClusterIP: "10.0.0.1"}}
	mutateService(service, core, mcpPortName, mcpPort(core))

	if service.Spec.Type != corev1.ServiceTypeClusterIP || service.Spec.ClusterIP != "10.0.0.1" {
		t.Errorf("service type/clusterIP = %s/%s", service.Spec.Type, service.Spec.ClusterIP)
	}
	want := corev1.ServicePort{Name: "mcp", Protocol: corev1.ProtocolTCP, Port: 9191, TargetPort: intstr.FromString("mcp")}
	if len(service.Spec.Ports) != 1 || service.Spec.Ports[0] != want {
		t.Errorf("ports = %+v, want %+v", service.Spec.Ports, want)
	}
	if !maps.Equal(service.Spec.Selector, coreSelectorLabels()) {
		t.Errorf("selector = %v, want %v", service.Spec.Selector, coreSelectorLabels())
	}
}

func TestNewDataPVC(t *testing.T) {
	core := testCore()
	pvc := newDataPVC(core, "test")

	if pvc.Name != "rhaf-core-data" || pvc.Namespace != "test" {
		t.Errorf("pvc = %s/%s", pvc.Namespace, pvc.Name)
	}
	if len(pvc.OwnerReferences) != 0 {
		t.Errorf("pvc owner references = %v, want none", pvc.OwnerReferences)
	}
	if size := pvc.Spec.Resources.Requests[corev1.ResourceStorage]; size.Cmp(resource.MustParse("100Mi")) != 0 {
		t.Errorf("size = %s, want 100Mi", size.String())
	}
	if pvc.Spec.StorageClassName != nil {
		t.Errorf("storage class = %v, want cluster default", *pvc.Spec.StorageClassName)
	}
	if !slices.Equal(pvc.Spec.AccessModes, []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}) {
		t.Errorf("access modes = %v", pvc.Spec.AccessModes)
	}

	core.Spec.Storage = rhafv1alpha1.RHAFPortalCoreStorageSpec{Size: resource.MustParse("5Gi"), StorageClassName: "fast"}
	pvc = newDataPVC(core, "test")
	if size := pvc.Spec.Resources.Requests[corev1.ResourceStorage]; size.Cmp(resource.MustParse("5Gi")) != 0 {
		t.Errorf("size = %s, want 5Gi", size.String())
	}
	if pvc.Spec.StorageClassName == nil || *pvc.Spec.StorageClassName != "fast" {
		t.Errorf("storage class = %v, want fast", pvc.Spec.StorageClassName)
	}
}

func TestWanakuConfigHash(t *testing.T) {
	if wanakuConfigHash("a: 1") == wanakuConfigHash("a: 2") {
		t.Fatal("hash does not change with content")
	}
	if wanakuConfigHash("a: 1") != wanakuConfigHash("a: 1") {
		t.Fatal("hash is not stable")
	}
}

func TestMutateDeploymentPodSecurityContext(t *testing.T) {
	core := testCore()
	deployment := &appsv1.Deployment{}
	mutateDeployment(deployment, core, "")
	security := deployment.Spec.Template.Spec.SecurityContext
	if security == nil || security.RunAsNonRoot == nil || !*security.RunAsNonRoot || security.FSGroup != nil || security.RunAsUser != nil ||
		security.SeccompProfile == nil || security.SeccompProfile.Type != corev1.SeccompProfileTypeRuntimeDefault {
		t.Errorf("default pod security context = %+v, want runAsNonRoot and RuntimeDefault seccomp only", security)
	}

	// User settings are kept; runAsNonRoot is always enforced.
	core.Spec.PodSecurityContext = &corev1.PodSecurityContext{FSGroup: new(int64(1001)), RunAsNonRoot: new(false)}
	mutateDeployment(deployment, core, "")
	security = deployment.Spec.Template.Spec.SecurityContext
	if security == nil || security.FSGroup == nil || *security.FSGroup != 1001 || security.RunAsNonRoot == nil || !*security.RunAsNonRoot {
		t.Errorf("pod security context = %+v, want fsGroup 1001 and runAsNonRoot", security)
	}
	if core.Spec.PodSecurityContext.RunAsNonRoot == nil || *core.Spec.PodSecurityContext.RunAsNonRoot {
		t.Error("mutateDeployment modified the spec")
	}
}

func TestMutateDeploymentAdoptionKeepsSelectorMatchingTemplate(t *testing.T) {
	previous := testCore()
	previous.Name = "previous"
	deployment := &appsv1.Deployment{}
	mutateDeployment(deployment, previous, "")

	current := testCore()
	mutateDeployment(deployment, current, "")

	selector, err := metav1.LabelSelectorAsSelector(deployment.Spec.Selector)
	if err != nil {
		t.Fatalf("selector: %v", err)
	}
	if !selector.Matches(labels.Set(deployment.Spec.Template.Labels)) {
		t.Fatalf("selector %v does not match template labels %v", deployment.Spec.Selector, deployment.Spec.Template.Labels)
	}
	if instance := deployment.Spec.Template.Labels["app.kubernetes.io/instance"]; instance != current.Name {
		t.Errorf("template instance label = %q, want %q", instance, current.Name)
	}
}

func TestMutateDeploymentHonorsExplicitSeccompProfile(t *testing.T) {
	core := testCore()
	core.Spec.PodSecurityContext = &corev1.PodSecurityContext{SeccompProfile: &corev1.SeccompProfile{
		Type: corev1.SeccompProfileTypeLocalhost, LocalhostProfile: new("profiles/core.json"),
	}}
	deployment := &appsv1.Deployment{}
	mutateDeployment(deployment, core, "")

	pod := deployment.Spec.Template.Spec.SecurityContext
	if pod == nil || pod.SeccompProfile == nil || pod.SeccompProfile.Type != corev1.SeccompProfileTypeLocalhost ||
		pod.SeccompProfile.LocalhostProfile == nil || *pod.SeccompProfile.LocalhostProfile != "profiles/core.json" {
		t.Errorf("pod seccomp profile = %+v, want Localhost profiles/core.json", pod)
	}
	if container := coreContainer(t, deployment); container.SecurityContext.SeccompProfile != nil {
		t.Errorf("container seccomp profile = %+v, want inherited from the pod", container.SecurityContext.SeccompProfile)
	}
}
