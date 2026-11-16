// Copyright 2026 The RHAF Portal Operator Authors
// SPDX-License-Identifier: Apache-2.0

// Command rhaf-portal-operator runs the RHAF Portal Operator.
package main

import (
	"flag"
	"fmt"
	"os"

	rhafv1alpha1 "github.com/rhaf-ai/rhaf-portal-operator/api/v1alpha1"
	"github.com/rhaf-ai/rhaf-portal-operator/controllers"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

func main() {
	var metricsAddress string
	var probeAddress string
	var watchNamespace string
	flag.StringVar(&metricsAddress, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddress, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.StringVar(&watchNamespace, "watch-namespace", "", "Namespace to watch. Watches all namespaces when empty.")
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&zap.Options{Development: true})))
	scheme := runtime.NewScheme()
	must(clientgoscheme.AddToScheme(scheme))
	must(rhafv1alpha1.AddToScheme(scheme))

	config, err := ctrl.GetConfig()
	must(err)
	options := ctrl.Options{
		Scheme:                 scheme,
		LeaderElection:         false,
		Metrics:                metricsserver.Options{BindAddress: metricsAddress},
		HealthProbeBindAddress: probeAddress,
	}
	if watchNamespace != "" {
		options.Cache = cache.Options{DefaultNamespaces: map[string]cache.Config{watchNamespace: {}}}
	}

	manager, err := ctrl.NewManager(config, options)
	must(err)

	must((&controllers.RHAFPortalCoreReconciler{Client: manager.GetClient()}).SetupWithManager(manager))
	must((&controllers.RHAFMCPServerReconciler{Client: manager.GetClient()}).SetupWithManager(manager))
	must(manager.AddHealthzCheck("healthz", healthz.Ping))
	must(manager.AddReadyzCheck("readyz", healthz.Ping))

	if err := manager.Start(ctrl.SetupSignalHandler()); err != nil {
		fmt.Fprintf(os.Stderr, "run manager: %v\n", err)
		os.Exit(1)
	}
}

func must(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
