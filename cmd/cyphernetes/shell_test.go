package main

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/avitaltamir/cyphernetes/pkg/parser"
	"github.com/chzyer/readline"
)

func TestShellPrompt(t *testing.T) {
	// Save the original namespace and restore it after the test
	originalNamespace := parser.Namespace
	defer func() { parser.Namespace = originalNamespace }()

	tests := []struct {
		name      string
		namespace string
		want      string
	}{
		{"Default namespace", "default", "\\033\\[32m\\(.*\\) default »\\033\\[0m "},
		{"Custom namespace", "custom-ns", "\\033\\[32m\\(.*\\) custom-ns »\\033\\[0m "},
		{"All namespaces", "", "\\033\\[31m\\(.*\\) ALL NAMESPACES »\\033\\[0m "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser.Namespace = tt.namespace
			got := shellPrompt()
			if !regexp.MustCompile(tt.want).MatchString(got) {
				t.Errorf("shellPrompt() = %v, does not match regex %v", got, tt.want)
			}
		})
	}
}

func TestGetCurrentContext(t *testing.T) {
	originalFunc := getCurrentContextFunc
	defer func() { getCurrentContextFunc = originalFunc }()

	getCurrentContextFunc = func() (string, error) {
		return "test-context", nil
	}

	context, err := getCurrentContext()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if context != "test-context" {
		t.Errorf("Expected context 'test-context', got '%s'", context)
	}

	getCurrentContextFunc = func() (string, error) {
		return "", fmt.Errorf("test error")
	}

	_, err = getCurrentContext()
	if err == nil {
		t.Error("Expected error, got nil")
	}
}

func TestFilterInput(t *testing.T) {
	tests := []struct {
		name     string
		input    rune
		expected bool
	}{
		{"Normal character", 'a', true},
		{"CtrlZ", readline.CharCtrlZ, false},
		{"Other control character", readline.CharCtrlH, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, got := filterInput(tt.input)
			if got != tt.expected {
				t.Errorf("filterInput(%v) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}
