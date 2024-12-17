package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"
)

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

func TestVersionOutput(t *testing.T) {
	// Save original stdout and Version
	oldStdout := os.Stdout
	originalVersion := Version
	defer func() {
		os.Stdout = oldStdout
		Version = originalVersion
	}()

	// Set test version
	Version = "test-version"

	// Capture stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Save original args
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	// Set up args for this test
	os.Args = []string{"cyphernetes", "--version"}

	// Execute command
	// Use a goroutine to handle os.Exit
	exit := make(chan int, 1)
	patch := func(code int) { exit <- code }
	originalExit := osExit
	osExit = patch
	defer func() { osExit = originalExit }()

	go func() {
		Execute()
		exit <- 0
	}()

	// Restore stdout and read output
	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	expectedLines := []string{
		"Cyphernetes test-version",
		fmt.Sprintf("Go Version: %s", runtime.Version()),
		"License: Apache 2.0",
		"Source: https://github.com/avitaltamir/cyphernetes",
	}
	for _, expectedLine := range expectedLines {
		if !strings.Contains(output, expectedLine) {
			t.Errorf("Expected output to contain %q, but it didn't.\nGot: %s", expectedLine, output)
		}
	}

	// Check that we would have exited with code 0
	select {
	case code := <-exit:
		if code != 0 {
			t.Errorf("Expected exit code 0, got %d", code)
		}
	case <-time.After(time.Second):
		t.Error("Test timed out")
	}
}

// osExit is used to mock os.Exit in tests
var osExit = os.Exit
