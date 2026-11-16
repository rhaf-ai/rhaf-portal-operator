// Copyright 2026 The RHAF Portal Operator Authors
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"slices"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const finalizerName = "rhaf.ai.redhat.com/finalizer"

func reconcileFinalizer(ctx context.Context, kubeClient client.Client, object client.Object) error {
	if object.GetDeletionTimestamp().IsZero() {
		if !containsString(object.GetFinalizers(), finalizerName) {
			object.SetFinalizers(append(object.GetFinalizers(), finalizerName))

			return kubeClient.Update(ctx, object)
		}

		return nil
	}

	if containsString(object.GetFinalizers(), finalizerName) {
		object.SetFinalizers(removeString(object.GetFinalizers(), finalizerName))

		return kubeClient.Update(ctx, object)
	}

	return nil
}

func containsString(values []string, value string) bool {
	return slices.Contains(values, value)
}

func removeString(values []string, value string) []string {
	result := values[:0]
	for _, candidate := range values {
		if candidate != value {
			result = append(result, candidate)
		}
	}

	return result
}
