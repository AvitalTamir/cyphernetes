package main

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/avitaltamir/cyphernetes/pkg/parser"
	"github.com/wader/readline"
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

	getCurrentContextFunc = func() (string, string, error) {
		return "test-context", "test-namespace", nil
	}

	context, namespace, err := getCurrentContext()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if context != "test-context" {
		t.Errorf("Expected context 'test-context', got '%s'", context)
	}
	if namespace != "test-namespace" {
		t.Errorf("Expected namespace 'test-namespace', got '%s'", namespace)
	}

	getCurrentContextFunc = func() (string, string, error) {
		return "", "", fmt.Errorf("test error")
	}

	_, _, err = getCurrentContext()
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

func TestSyntaxHighlighterPaint(t *testing.T) {
	h := &syntaxHighlighter{}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Keywords",
			input:    "MATCH (n:Node) WHERE n.property = 'value' RETURN n",
			expected: "\x1b[35mMATCH\x1b[0m \x1b[37m(\x1b[\x1b[33mn\x1b[0m:\x1b[94mNode\x1b[0m\x1b[37m)\x1b[0m \x1b[35mWHERE\x1b[0m n.property = 'value' \x1b[35mRETURN n\x1b[0m",
		},
		{
			name:     "Properties",
			input:    "MATCH (n:Node {key: \"value\"})",
			expected: "\x1b[35mMATCH\x1b[0m \x1b[37m(\x1b[\x1b[33mn\x1b[0m:\x1b[94mNode\x1b[0m \x1b[37m{\x1b[33mkey: \x1b[0m\x1b[36m\"value\"\x1b[0m}\x1b[0m\x1b[37m)\x1b[0m\x1b[0m",
		},
		{
			name:     "Return with JSONPath",
			input:    "RETURN n.name, n.age",
			expected: "\x1b[35mRETURN n\x1b[37m.\x1b[35mname\x1b[37m,\x1b[35m n\x1b[37m.\x1b[35mage\x1b[0m",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := string(h.Paint([]rune(tt.input), 0))
			if result != tt.expected {
				t.Errorf("\nPaint() = %v\n   want = %v\n    raw = %#v\n   want = %#v", result, tt.expected, result, tt.expected)
			}
		})
	}
}

func TestExecuteMacro(t *testing.T) {
	// Create a new MacroManager
	mm := NewMacroManager()

	// Add a test macro
	testMacro := &Macro{
		Name:        "testMacro",
		Args:        []string{"arg1"},
		Statements:  []string{"MATCH (n:$arg1) RETURN n"},
		Description: "Test macro",
	}
	mm.AddMacro(testMacro, false)

	// Set the global macroManager to our test instance
	macroManager = mm

	tests := []struct {
		name           string
		macroName      string
		args           []string
		expectedResult string
		expectError    bool
	}{
		{
			name:           "Macro not found",
			macroName:      "nonExistentMacro",
			args:           []string{},
			expectedResult: "",
			expectError:    true,
		},
		{
			name:           "Incorrect argument count",
			macroName:      "testMacro",
			args:           []string{},
			expectedResult: "",
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := executeMacro(":" + tt.macroName + " " + strings.Join(tt.args, " "))

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected an error, but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if result != tt.expectedResult {
					t.Errorf("Expected result %q, but got %q", tt.expectedResult, result)
				}
			}
		})
	}
}
