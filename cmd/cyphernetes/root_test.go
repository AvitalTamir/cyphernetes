package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/avitaltamir/cyphernetes/pkg/core"
)

func TestContextFlagRegistered(t *testing.T) {
	flag := rootCmd.PersistentFlags().Lookup("context")
	if flag == nil {
		t.Fatal("expected --context persistent flag to be registered")
	}
	if flag.DefValue != "" {
		t.Errorf("expected --context default to be empty, got %q", flag.DefValue)
	}
}

func TestContextFlagBindsToCoreKubeContext(t *testing.T) {
	original := core.KubeContext
	defer func() {
		core.KubeContext = original
		// Reset the flag value so other tests start clean.
		_ = rootCmd.PersistentFlags().Set("context", original)
	}()

	if err := rootCmd.PersistentFlags().Set("context", "my-context"); err != nil {
		t.Fatalf("failed to set --context flag: %v", err)
	}
	if core.KubeContext != "my-context" {
		t.Errorf("expected core.KubeContext to be %q, got %q", "my-context", core.KubeContext)
	}
}

func TestExecuteNoArgs(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Test the Execute function
	Execute()

	// Restore stdout
	w.Close()
	os.Stdout = oldStdout

	// Read the captured output
	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// Check if the output contains expected content
	expectedContent := "Use \"cyphernetes [command] --help\" for more information about a command."
	if !strings.Contains(output, expectedContent) {
		t.Errorf("Execute() output does not contain expected content.\nExpected: %s\nGot: %s", expectedContent, output)
	}
}

func TestExecuteWithArgs(t *testing.T) {
	testCases := []struct {
		name          string
		args          []string
		expectedError bool
	}{
		{"No args", []string{}, false},
		{"Help flag", []string{"--help"}, false},
		{"Invalid flag", []string{"--invalid-flag"}, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := TestExecute(tc.args)
			if tc.expectedError && err == nil {
				t.Errorf("Expected an error, but got none")
			}
			if !tc.expectedError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}
