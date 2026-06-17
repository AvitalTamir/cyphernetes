package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/avitaltamir/cyphernetes/pkg/core"
	"github.com/wader/readline"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
)

func TestShellPrompt(t *testing.T) {
	// Save the original namespace and restore it after the test
	originalNamespace := core.Namespace
	defer func() { core.Namespace = originalNamespace }()

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
			core.Namespace = tt.namespace
			got := shellPrompt()
			if !regexp.MustCompile(tt.want).MatchString(got) {
				t.Errorf("shellPrompt() = %v, does not match regex %v", got, tt.want)
			}
		})
	}
}

func TestShellPromptNoColor(t *testing.T) {
	// Save the original namespace and noColor options and restore it after the test
	originalNamespace := core.Namespace
	originalNoColor := core.NoColor
	defer func() {
		core.Namespace = originalNamespace
		core.NoColor = originalNoColor
	}()

	tests := []struct {
		name      string
		namespace string
		want      string
	}{
		{"Default namespace", "default", "\\(.*\\) default » "},
		{"Custom namespace", "custom-ns", "\\(.*\\) custom-ns »"},
		{"All namespaces", "", "\\(.*\\) ALL NAMESPACES » "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			core.NoColor = true
			core.Namespace = tt.namespace
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

func TestToggleVimMode(t *testing.T) {
	var out bytes.Buffer
	rl, err := readline.NewEx(&readline.Config{
		Stdin:          io.NopCloser(strings.NewReader("")),
		Stdout:         &out,
		Stderr:         &out,
		FuncIsTerminal: func() bool { return false },
		FuncMakeRaw:    func() error { return nil },
		FuncExitRaw:    func() error { return nil },
	})
	if err != nil {
		t.Fatalf("failed to create readline instance: %v", err)
	}
	defer rl.Close()

	if rl.IsVimMode() {
		t.Fatal("expected Vim mode to be disabled by default")
	}
	if enabled := toggleVimMode(rl); !enabled || !rl.IsVimMode() {
		t.Fatalf("expected first toggle to enable Vim mode, got enabled=%t IsVimMode=%t", enabled, rl.IsVimMode())
	}
	if enabled := toggleVimMode(rl); enabled || rl.IsVimMode() {
		t.Fatalf("expected second toggle to disable Vim mode, got enabled=%t IsVimMode=%t", enabled, rl.IsVimMode())
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
			input:    `MATCH (n:Node) WHERE n.property = "value" RETURN n`,
			expected: "\x1b[35mMATCH\x1b[0m \x1b[37m(\x1b[0m\x1b[33mn\x1b[0m:\x1b[94mNode\x1b[0m\x1b[37m)\x1b[0m \x1b[35mWHERE\x1b[0m \x1b[33mn\x1b[0m\x1b[35m.\x1b[0mproperty \x1b[90m=\x1b[0m \x1b[36m\"value\"\x1b[0m \x1b[35mRETURN\x1b[0m n",
		},
		{
			name:     "Properties",
			input:    `MATCH (n:Node {key: "value"})`,
			expected: "\x1b[35mMATCH\x1b[0m \x1b[37m(\x1b[0m\x1b[33mn\x1b[0m:\x1b[94mNode \x1b[37m{\x1b[0m\x1b[33mkey: \x1b[0m\x1b[36m\x1b[36m\"value\"\x1b[0m\x1b[0m\x1b[37m}\x1b[0m\x1b[0m\x1b[37m)\x1b[0m",
		},
		{
			name:     "Return with JSONPath",
			input:    "RETURN n.name, n.age",
			expected: "\x1b[35mRETURN\x1b[0m \x1b[33mn\x1b[0m\x1b[35m.\x1b[0mname, \x1b[33mn\x1b[0m\x1b[35m.\x1b[0mage",
		},
		{
			name:     "Multi return with JSONPaths and aliases",
			input:    "RETURN n.name, n.age as age, n.email",
			expected: "\x1b[35mRETURN\x1b[0m \x1b[33mn\x1b[0m\x1b[35m.\x1b[0mname, \x1b[33mn\x1b[0m\x1b[35m.\x1b[0mage \x1b[35mAS\x1b[0m age, \x1b[33mn\x1b[0m\x1b[35m.\x1b[0memail",
		},
		{
			name:     "Return with JSONPath and asterisk",
			input:    "RETURN n.*",
			expected: "\x1b[35mRETURN\x1b[0m n.*",
		},
		{
			name:     "Kindless node",
			input:    "MATCH (:Pod) RETURN n",
			expected: "\x1b[35mMATCH\x1b[0m \x1b[37m(\x1b[0m:\x1b[94mPod\x1b[0m\x1b[37m)\x1b[0m \x1b[35mRETURN\x1b[0m n",
		},
		{
			name:     "Anonymous node",
			input:    "MATCH (pod) RETURN pod",
			expected: "\x1b[35mMATCH\x1b[0m \x1b[37m(\x1b[0m\x1b[33mpod\x1b[0m\x1b[37m)\x1b[0m \x1b[35mRETURN\x1b[0m pod",
		},
		{
			name:     "Mixed nodes",
			input:    "MATCH (pod)-[:EXPOSED_BY]->(:Service) RETURN pod",
			expected: "\x1b[35mMATCH\x1b[0m \x1b[37m(\x1b[0m\x1b[33mpod\x1b[0m\x1b[37m)\x1b[0m-\x1b[37m[\x1b[0m\x1b[94m:EXPOSED_BY\x1b[0m\x1b[37m]\x1b[0m->\x1b[37m(\x1b[0m:\x1b[94mService\x1b[0m\x1b[37m)\x1b[0m \x1b[35mRETURN\x1b[0m pod",
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

func TestSyntaxHighlighterComments(t *testing.T) {
	h := &syntaxHighlighter{}

	// Cases with a precisely known full output.
	exact := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "pure single-line comment",
			input:    "// hello world",
			expected: "\x1b[90m// hello world\x1b[0m",
		},
		{
			name:     "pure multi-line comment",
			input:    "/* a comment */",
			expected: "\x1b[90m/* a comment */\x1b[0m",
		},
		{
			name:     "code followed by single-line comment",
			input:    "RETURN n // get n",
			expected: "\x1b[35mRETURN\x1b[0m n \x1b[90m// get n\x1b[0m",
		},
	}
	for _, tt := range exact {
		t.Run(tt.name, func(t *testing.T) {
			result := string(h.Paint([]rune(tt.input), 0))
			if result != tt.expected {
				t.Errorf("\nPaint() = %#v\n   want = %#v", result, tt.expected)
			}
		})
	}

	// Cases where we assert key substrings (the full output is verbose).
	substr := []struct {
		name           string
		input          string
		mustContain    []string
		mustNotContain []string
	}{
		{
			name:        "inline multi-line comment between code",
			input:       "MATCH /* x */ (n) RETURN n",
			mustContain: []string{"\x1b[90m/* x */\x1b[0m", "\x1b[35mMATCH\x1b[0m", "\x1b[35mRETURN\x1b[0m"},
		},
		{
			name:        "unterminated multi-line comment colors to end of line",
			input:       "MATCH /* oops",
			mustContain: []string{"\x1b[90m/* oops\x1b[0m", "\x1b[35mMATCH\x1b[0m"},
		},
		{
			name:           "slashes inside a string are not treated as a comment",
			input:          `WHERE n.name = "http://x"`,
			mustContain:    []string{"\x1b[36m\"http://x\"\x1b[0m"},
			mustNotContain: []string{"\x1b[90m//"},
		},
	}
	for _, tt := range substr {
		t.Run(tt.name, func(t *testing.T) {
			result := string(h.Paint([]rune(tt.input), 0))
			for _, s := range tt.mustContain {
				if !strings.Contains(result, s) {
					t.Errorf("Paint() = %#v\n   missing %#v", result, s)
				}
			}
			for _, s := range tt.mustNotContain {
				if strings.Contains(result, s) {
					t.Errorf("Paint() = %#v\n   should not contain %#v", result, s)
				}
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

func Test_listRelationshipRules(t *testing.T) {
	tests := []struct {
		name    string
		want    bool
		wantErr bool
	}{
		{
			name:    "Success",
			want:    true,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := listRelationshipRules()
			if (err != nil) != tt.wantErr {
				t.Errorf("listRelationshipRules() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if validateJSONArray(got) != tt.want {
				t.Errorf("listRelationshipRules() = %v, invalid json", got)
			}
		})
	}
}

func validateJSONArray(input string) bool {
	// Check if this is a valid JSON array
	var items []string
	if err := json.Unmarshal([]byte(input), &items); err != nil {
		return false
	}

	// Regular expression pattern to validate each item
	// Format: "ALPHANUMERIC_WITH_UNDERSCORES" or "ALPHANUMERIC_WITH_UNDERSCORES_AND_INSPEC"
	pattern := `^[A-Z]+(_[A-Z]+)*(?:_INSPEC_[A-Z]+)?$`
	re := regexp.MustCompile(pattern)

	for _, item := range items {
		if !re.MatchString(item) {
			return false
		}
	}

	return true
}

func Test_describeRelationshipRule(t *testing.T) {
	type args struct {
		input string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name:    string(core.GetRelationshipRules()[0].Relationship),
			args:    args{input: string(core.GetRelationshipRules()[0].Relationship)},
			want:    "{\"KindA\":\"pods\",\"KindB\":\"replicasets\",\"Relationship\":\"REPLICASET_OWN_POD\",\"MatchCriteria\":[{\"FieldA\":\"$.metadata.ownerReferences[].name\",\"FieldB\":\"$.metadata.name\",\"ComparisonType\":\"ExactMatch\",\"DefaultProps\":null}]}",
			wantErr: false,
		},
		{
			name:    "non-existatnt",
			args:    args{input: "non-existatnt"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := describeRelationshipRule(tt.args.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("describeRelationshipRule() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("describeRelationshipRule() = %v, want %v", got, tt.want)
			}
		})
	}
}

const shellTestKubeconfig = `apiVersion: v1
kind: Config
current-context: ctx-a
clusters:
- name: cluster-a
  cluster:
    server: https://server-a.example.com
- name: cluster-b
  cluster:
    server: https://server-b.example.com
contexts:
- name: ctx-a
  context:
    cluster: cluster-a
    user: user-a
    namespace: ns-a
- name: ctx-b
  context:
    cluster: cluster-b
    user: user-b
    namespace: ns-b
users:
- name: user-a
  user:
    token: token-a
- name: user-b
  user:
    token: token-b
`

func TestGetCurrentContextFromConfigHonorsKubeContext(t *testing.T) {
	dir := t.TempDir()
	kubeconfigPath := filepath.Join(dir, "config")
	if err := os.WriteFile(kubeconfigPath, []byte(shellTestKubeconfig), 0o600); err != nil {
		t.Fatalf("failed to write temp kubeconfig: %v", err)
	}
	t.Setenv("KUBECONFIG", kubeconfigPath)

	originalKubeContext := core.KubeContext
	defer func() { core.KubeContext = originalKubeContext }()

	tests := []struct {
		name        string
		kubeContext string
		wantContext string
		wantNs      string
	}{
		{"defaults to current-context", "", "ctx-a", "ns-a"},
		{"honors --context override", "ctx-b", "ctx-b", "ns-b"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			core.KubeContext = tt.kubeContext
			gotContext, gotNs, err := getCurrentContextFromConfig()
			if err != nil {
				t.Fatalf("getCurrentContextFromConfig() returned error: %v", err)
			}
			if gotContext != tt.wantContext {
				t.Errorf("context = %q, want %q", gotContext, tt.wantContext)
			}
			if gotNs != tt.wantNs {
				t.Errorf("namespace = %q, want %q", gotNs, tt.wantNs)
			}
		})
	}
}
