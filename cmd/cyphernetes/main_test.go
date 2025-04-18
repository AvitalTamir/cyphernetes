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

func checkOutput(t *testing.T, output, expectedContent, testName string, negate bool) {
	if negate {
		if strings.Contains(output, expectedContent) {
			t.Errorf("%s output contains unexpected content.\nExpected: %s\nGot: %s", testName, expectedContent, output)
		}
	} else {
		if !strings.Contains(output, expectedContent) {
			t.Errorf("%s output does not contain expected content.\nExpected: %s\nGot: %s", testName, expectedContent, output)
		}
	}
}

func checkPrompt(t *testing.T, output, expectedPromptOutput, testName string) {
	if !regexp.MustCompile(expectedPromptOutput).MatchString(output) {
		t.Errorf("%s shell prompt does not contain expected prompt.\nExpected: %s\nGot: %s", testName, expectedPromptOutput, output)
	}
}

func TestMain(t *testing.T) {
	stdout, _ := runTestCommand(t, "TestMainHelper", "TEST_MAIN")
	checkOutput(t, stdout, "Use \"cyphernetes [command] --help\" for more information about a command.", "Execute()", false)
}

func TestMainHelper(t *testing.T) {
	if os.Getenv("TEST_MAIN") != "1" {
		return
	}
	main()
}

func TestCyphernetesShellNoFlag(t *testing.T) {
	stdout, _ := runTestCommand(t, "TestCyphernetesShellNoFlagHelper", "TEST_SHELL_NO_FLAG")
	checkOutput(t, stdout, "Type 'exit' or press Ctrl-D to exit\nType 'help' for information on how to use the shell\n", "\"cyphernetes shell\"", false)
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
	checkOutput(t, stdout, "Type 'exit' or press Ctrl-D to exit\nType 'help' for information on how to use the shell\n", "\"cyphernetes shell -A\"", false)
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

func TestCyphernetesShellWithHelpFlagHelper(t *testing.T) {
	if os.Getenv("TEST_SHELL_HELP") != "1" {
		return
	}
	os.Args = []string{"cyphernetes", "shell", "-h"}
	main()
}

func TestCyphernetesShellWithNamespaceFlag(t *testing.T) {
	stdout, _ := runTestCommand(t, "TestCyphernetesShellWithNamespaceFlagHelper", "TEST_SHELL_NAMESPACE")
	checkOutput(t, stdout, "Type 'exit' or press Ctrl-D to exit\nType 'help' for information on how to use the shell\n", "\"cyphernetes shell -n custom-namespace\"", false)
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
	stdout, _ := runTestCommand(t, "TestCyphernetesShellWithLogLevelFlagHelper", "TEST_SHELL_LOG_LEVEL")
	checkOutput(t, stdout, "[DEBUG]", "\"cyphernetes shell -l debug\"", false)
}

func TestCyphernetesShellWithLogLevelFlagHelper(t *testing.T) {
	if os.Getenv("TEST_SHELL_LOG_LEVEL") != "1" {
		return
	}
	os.Args = []string{"cyphernetes", "shell", "-l", "debug"}
	main()
}

func TestCyphernetesShellNoColorFlag(t *testing.T) {
	stdout, _ := runTestCommand(t, "TestCyphernetesShellNoColorFlagHelper", "TEST_SHELL_NO_COLOR")
	checkOutput(t, stdout, "Type 'exit' or press Ctrl-D to exit\nType 'help' for information on how to use the shell\n", "\"cyphernetes shell --no-color\"", false)
	checkPrompt(t, stdout, "(.*) default » ", "\"cyphernetes shell --no-color\"")
}

func TestCyphernetesShellNoColorFlagHelper(t *testing.T) {
	if os.Getenv("TEST_SHELL_NO_COLOR") != "1" {
		return
	}
	os.Args = []string{"cyphernetes", "shell", "--no-color"}
	main()
	fmt.Print(shellPrompt())
}

func TestCyphernetesQueryQuietMode(t *testing.T) {
	stdout, _ := runTestCommand(t, "TestCyphernetesQueryQuietModeHelper", "TEST_CYPHERNETES_QUERY_QUIET_MODE")
	checkOutput(t, stdout, "Initializing relationships", "\"cyphernetes query 'match (p:pod) return p'\"", true)
}

func TestCyphernetesQueryQuietModeHelper(t *testing.T) {
	if os.Getenv("TEST_CYPHERNETES_QUERY_QUIET_MODE") != "1" {
		return
	}
	os.Args = []string{"cyphernetes", "query", "'match (p:pod) return p'"}
	main()
}

func TestCyphernetesVersion(t *testing.T) {
	stdout, _ := runTestCommand(t, "TestCyphernetesVersionHelper", "TEST_CYPHERNETES_VERSION")
	checkOutput(t, stdout, "Cyphernetes dev\nLicense: Apache 2.0\nSource: https://github.com/avitaltamir/cyphernetes\n", "\"cyphernetes --version\"", false)
}

func TestCyphernetesVersionHelper(t *testing.T) {
	if os.Getenv("TEST_CYPHERNETES_VERSION") != "1" {
		return
	}
	os.Args = []string{"cyphernetes", "--version"}
	main()
}
