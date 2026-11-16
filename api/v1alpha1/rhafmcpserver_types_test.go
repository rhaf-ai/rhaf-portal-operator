// Copyright 2026 The RHAF Portal Operator Authors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestNamespaceBasePattern(t *testing.T) {
	pattern := regexp.MustCompile(NamespaceBasePattern)

	for _, value := range []string{"a", "a1", "valid-name", strings.Repeat("a", 51)} {
		if !pattern.MatchString(value) {
			t.Errorf("pattern does not accept %q", value)
		}
	}

	for _, value := range []string{"", "-starts", "ends-", "Uppercase", "has_underscore", strings.Repeat("a", 52)} {
		if pattern.MatchString(value) {
			t.Errorf("pattern accepts invalid value %q", value)
		}
	}
}

func TestGeneratedCRDUsesNamespaceBasePattern(t *testing.T) {
	contents, err := os.ReadFile("../../config/crd/bases/rhaf.ai.redhat.com_rhafmcpservers.yaml")
	if err != nil {
		t.Fatalf("read generated CRD: %v", err)
	}

	if !strings.Contains(string(contents), "pattern: "+NamespaceBasePattern) {
		t.Fatalf("generated CRD does not contain namespaceBase pattern %q", NamespaceBasePattern)
	}
}
