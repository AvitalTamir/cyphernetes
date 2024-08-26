package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestMain(t *testing.T) {
	// Create a temporary file to capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Test the Execute function
	main()

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
