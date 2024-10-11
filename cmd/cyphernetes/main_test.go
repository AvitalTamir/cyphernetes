package main

import (
	"bytes"
	"io"
	"os"
	"regexp"
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

func TestCyphernetesShellNoFlag(t *testing.T) {
	// Create a temporary file to capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Simulate CLI input
	os.Args = []string{"cyphernetes", "shell"}
	shellCommandString := strings.Join(os.Args, " ")

	// Test the Execute function
	main()

	// get the shell prompt
	promptOutput := shellPrompt()

	// Restore stdout
	w.Close()
	os.Stdout = oldStdout

	// Read the captured output
	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// Check if the output contains expected content
	expectedContent := "Type 'exit' or press Ctrl-D to exit\nType 'help' for information on how to use the shell\n"
	if !strings.Contains(output, expectedContent) {
		t.Errorf("\"%s\" output does not contain expected content.\nExpected: %s\nGot: %s", shellCommandString, expectedContent, output)

	}

	expectedPromptOutput := "\\033\\[32m\\(.*\\) default »\\033\\[0m "
	if !regexp.MustCompile(expectedPromptOutput).MatchString(promptOutput) {
		t.Errorf("\"%s\" shell prompt does not contain expected prompt.\nExpected: %s\nGot: %s", shellCommandString, expectedPromptOutput, promptOutput)
	}
}

func TestCyphernetesShellWithAllNamespacesFlag(t *testing.T) {
	// Create a temporary file to capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Simulate CLI input
	os.Args = []string{"cyphernetes", "shell", "-A"}
	shellCommandString := strings.Join(os.Args, " ")

	// Test the Execute function
	main()

	// get the shell prompt
	promptOutput := shellPrompt()

	// Restore stdout
	w.Close()
	os.Stdout = oldStdout

	// Read the captured output
	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// Check if the output contains expected content
	expectedContent := "Type 'exit' or press Ctrl-D to exit\nType 'help' for information on how to use the shell\n"
	if !strings.Contains(output, expectedContent) {
		t.Errorf("\"%s\" output does not contain expected content.\nExpected: %s\nGot: %s", shellCommandString, expectedContent, output)
	}

	expectedPromptOutput := "\\033\\[31m\\(.*\\) ALL NAMESPACES »\\033\\[0m "
	if !regexp.MustCompile(expectedPromptOutput).MatchString(promptOutput) {
		t.Errorf("\"%s\" shell prompt does not contain expected prompt.\nExpected: %s\nGot: %s", shellCommandString, expectedPromptOutput, promptOutput)
	}
}

func TestCyphernetesShellWithHelpFlag(t *testing.T) {
	// Save the original os.Args
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	// Create a temporary file to capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Simulate CLI input
	os.Args = []string{"cyphernetes", "shell", "-h"}
	shellCommandString := strings.Join(os.Args, " ")

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
	expectedContent := `Launch an interactive shell

Usage:
  cyphernetes shell [flags]

Flags:
  -h, --help   help for shell

Global Flags:
  -A, --all-namespaces     Query all namespaces
  -l, --loglevel string    The log level to use (debug, info, warn, error, fatal, panic) (default "info")
  -n, --namespace string   The namespace to query against (default "default")`

	if !strings.Contains(output, expectedContent) {
		t.Errorf("\"%s\" output does not contain expected content.\nExpected: %s\nGot: %s", shellCommandString, expectedContent, output)
	}
}

func TestCyphernetesShellWithNamespaceFlag(t *testing.T) {
	// Create a temporary file to capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Simulate CLI input
	os.Args = []string{"cyphernetes", "shell", "-n", "custom-namespace"}
	shellCommandString := strings.Join(os.Args, " ")

	// Test the Execute function
	main()

	// get the shell prompt
	promptOutput := shellPrompt()

	// Restore stdout
	w.Close()
	os.Stdout = oldStdout

	// Read the captured output
	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// Check if the output contains expected content
	expectedContent := "Type 'exit' or press Ctrl-D to exit\nType 'help' for information on how to use the shell\n"
	if !strings.Contains(output, expectedContent) {
		t.Errorf("\"%s\" output does not contain expected content.\nExpected: %s\nGot: %s", shellCommandString, expectedContent, output)
	}

	expectedPromptOutput := "\\033\\[32m\\(.*\\) custom-namespace »\\033\\[0m "
	if !regexp.MustCompile(expectedPromptOutput).MatchString(promptOutput) {
		t.Errorf("\"%s\" shell prompt does not contain expected prompt.\nExpected: %s\nGot: %s", shellCommandString, expectedPromptOutput, promptOutput)
	}
}

func TestCyphernetesShellWithLogLevelFlag(t *testing.T) {
	// TODO: see issue-XXXX
}
