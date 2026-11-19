// Copyright 2026 The RHAF Portal Operator Authors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"os"
	"strings"
	"testing"
)

func TestGeneratedCoreCRDDefaultsAndValidation(t *testing.T) {
	contents, err := os.ReadFile("../../config/crd/bases/rhaf.ai.redhat.com_rhafportalcores.yaml")
	if err != nil {
		t.Fatalf("read generated CRD: %v", err)
	}

	crd := string(contents)
	for _, expected := range []string{
		"default: 8080",
		"default: 8081",
		"default: 100Mi",
		"rule: self.management.port != self.mcp.port",
		"configMapRef:",
		"podSecurityContext:",
		"maxLength: 253",
		"message: runAsNonRoot cannot be false",
		"message: runAsUser cannot be 0",
	} {
		if !strings.Contains(crd, expected) {
			t.Errorf("generated CRD does not contain %q", expected)
		}
	}
}
