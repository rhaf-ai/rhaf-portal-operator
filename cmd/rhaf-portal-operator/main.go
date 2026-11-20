// Copyright 2026 The RHAF Portal Operator Authors
// SPDX-License-Identifier: Apache-2.0

// Command rhaf-portal-operator runs the RHAF Portal Operator.
package main

import (
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"

	rhafv1alpha1 "github.com/rhaf-ai/rhaf-portal-operator/api/v1alpha1"
	"github.com/rhaf-ai/rhaf-portal-operator/controllers"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

func main() {
	var metricsAddress string
	var probeAddress string
	flag.StringVar(&metricsAddress, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddress, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.Parse()

	namespace := os.Getenv("POD_NAMESPACE")
	if namespace == "" {
		namespace = "rhaf-system"
	}

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&zap.Options{Development: true})))
	scheme := runtime.NewScheme()
	printIfError(clientgoscheme.AddToScheme(scheme))
	printIfError(rhafv1alpha1.AddToScheme(scheme))

	config, err := ctrl.GetConfig()
	printIfError(err)
	workloadCache := cache.ByObject{Namespaces: map[string]cache.Config{namespace: {}}}
	options := ctrl.Options{
		Scheme:                     scheme,
		LeaderElection:             true,
		LeaderElectionID:           "rhaf-portal-operator",
		LeaderElectionNamespace:    "kube-system",
		LeaderElectionResourceLock: "leases",
		Cache: cache.Options{ByObject: map[client.Object]cache.ByObject{
			&appsv1.Deployment{}:            workloadCache,
			&corev1.Service{}:               workloadCache,
			&corev1.PersistentVolumeClaim{}: workloadCache,
			&corev1.ConfigMap{}:             workloadCache,
		}},
		Metrics:                metricsserver.Options{BindAddress: metricsAddress},
		HealthProbeBindAddress: probeAddress,
	}
	manager, err := ctrl.NewManager(config, options)
	printIfError(err)

	printIfError((&controllers.RHAFPortalCoreReconciler{
		Client: manager.GetClient(), APIReader: manager.GetAPIReader(), WorkloadNamespace: namespace,
	}).SetupWithManager(manager))
	printIfError((&controllers.RHAFMCPServerReconciler{Client: manager.GetClient()}).SetupWithManager(manager))
	printIfError(manager.AddHealthzCheck("healthz", healthz.Ping))
	printIfError(manager.AddReadyzCheck("leader", func(_ *http.Request) error {
		select {
		case <-manager.Elected():
			return nil
		default:
			return errors.New("waiting for the operator Lease")
		}
	}))

	if err := manager.Start(ctrl.SetupSignalHandler()); err != nil {
		fmt.Fprintf(os.Stderr, "run manager: %v\n", err)
		os.Exit(1)
	}
}

func printIfError(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
