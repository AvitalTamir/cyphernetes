package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"
)

func runTestCommand(t *testing.T, testName, envVar string, args ...string) (string, string) {
	cmd := exec.Command(os.Args[0], append([]string{"-test.run=" + testName}, args...)...)
	cmd.Env = append(os.Environ(), envVar+"=1")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		t.Fatalf("Failed to run %s: %v", testName, err)
	}

	return stdout.String(), stderr.String()
}

func checkOutput(t *testing.T, output, expectedContent, testName string) {
	if !strings.Contains(output, expectedContent) {
		t.Errorf("%s output does not contain expected content.\nExpected: %s\nGot: %s", testName, expectedContent, output)
	}
}

func checkPrompt(t *testing.T, output, expectedPromptOutput, testName string) {
	if !regexp.MustCompile(expectedPromptOutput).MatchString(output) {
		t.Errorf("%s shell prompt does not contain expected prompt.\nExpected: %s\nGot: %s", testName, expectedPromptOutput, output)
	}
}

func TestMain(t *testing.T) {
	stdout, _ := runTestCommand(t, "TestMainHelper", "TEST_MAIN")
	checkOutput(t, stdout, "Use \"cyphernetes [command] --help\" for more information about a command.", "Execute()")
}

func TestMainHelper(t *testing.T) {
	if os.Getenv("TEST_MAIN") != "1" {
		return
	}
	main()
}

func TestCyphernetesShellNoFlag(t *testing.T) {
	stdout, _ := runTestCommand(t, "TestCyphernetesShellNoFlagHelper", "TEST_SHELL_NO_FLAG")
	checkOutput(t, stdout, "Type 'exit' or press Ctrl-D to exit\nType 'help' for information on how to use the shell\n", "\"cyphernetes shell\"")
	checkPrompt(t, stdout, "\\033\\[32m\\(.*\\) default »\\033\\[0m ", "\"cyphernetes shell\"")
}

func TestCyphernetesShellNoFlagHelper(t *testing.T) {
	if os.Getenv("TEST_SHELL_NO_FLAG") != "1" {
		return
	}
	os.Args = []string{"cyphernetes", "shell"}
	main()
	fmt.Print(shellPrompt())
}

func TestCyphernetesShellWithAllNamespacesFlag(t *testing.T) {
	stdout, _ := runTestCommand(t, "TestCyphernetesShellWithAllNamespacesFlagHelper", "TEST_SHELL_ALL_NAMESPACES")
	checkOutput(t, stdout, "Type 'exit' or press Ctrl-D to exit\nType 'help' for information on how to use the shell\n", "\"cyphernetes shell -A\"")
	checkPrompt(t, stdout, "\\033\\[31m\\(.*\\) ALL NAMESPACES »\\033\\[0m ", "\"cyphernetes shell -A\"")
}

func TestCyphernetesShellWithAllNamespacesFlagHelper(t *testing.T) {
	if os.Getenv("TEST_SHELL_ALL_NAMESPACES") != "1" {
		return
	}
	os.Args = []string{"cyphernetes", "shell", "-A"}
	main()
	fmt.Print(shellPrompt())
}

func TestCyphernetesShellWithHelpFlag(t *testing.T) {
	stdout, _ := runTestCommand(t, "TestCyphernetesShellWithHelpFlagHelper", "TEST_SHELL_HELP")
	expectedContent := `Launch an interactive shell

Usage:
  cyphernetes shell [flags]

Flags:
  -h, --help   help for shell

Global Flags:
  -A, --all-namespaces     Query all namespaces
  -l, --loglevel string    The log level to use (debug, info, warn, error, fatal, panic) (default "info")
  -n, --namespace string   The namespace to query against (default "default")`
	checkOutput(t, stdout, expectedContent, "\"cyphernetes shell -h\"")
}

func TestCyphernetesShellWithHelpFlagHelper(t *testing.T) {
	if os.Getenv("TEST_SHELL_HELP") != "1" {
		return
	}
	os.Args = []string{"cyphernetes", "shell", "-h"}
	main()
}

func TestCyphernetesShellWithNamespaceFlag(t *testing.T) {
	stdout, _ := runTestCommand(t, "TestCyphernetesShellWithNamespaceFlagHelper", "TEST_SHELL_NAMESPACE")
	checkOutput(t, stdout, "Type 'exit' or press Ctrl-D to exit\nType 'help' for information on how to use the shell\n", "\"cyphernetes shell -n custom-namespace\"")
	checkPrompt(t, stdout, "\\033\\[32m\\(.*\\) custom-namespace »\\033\\[0m ", "\"cyphernetes shell -n custom-namespace\"")
}

func TestCyphernetesShellWithNamespaceFlagHelper(t *testing.T) {
	if os.Getenv("TEST_SHELL_NAMESPACE") != "1" {
		return
	}
	os.Args = []string{"cyphernetes", "shell", "-n", "custom-namespace"}
	main()
	fmt.Print(shellPrompt())
}

func TestCyphernetesShellWithLogLevelFlag(t *testing.T) {
	_, stderr := runTestCommand(t, "TestCyphernetesShellWithLogLevelFlagHelper", "TEST_SHELL_LOG_LEVEL")
	checkOutput(t, stderr, "[DEBUG]", "\"cyphernetes shell -l debug\"")
}

func TestCyphernetesShellWithLogLevelFlagHelper(t *testing.T) {
	if os.Getenv("TEST_SHELL_LOG_LEVEL") != "1" || os.Getenv("CI") != "" {
		return
	}
	os.Args = []string{"cyphernetes", "shell", "-l", "debug"}
	main()
}
