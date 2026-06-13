package apiserver

import (
	"os"
	"path/filepath"
	"testing"
)

const twoContextKubeconfig = `apiVersion: v1
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

func writeTempKubeconfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	if err := os.WriteFile(path, []byte(twoContextKubeconfig), 0o600); err != nil {
		t.Fatalf("failed to write temp kubeconfig: %v", err)
	}
	return path
}

func TestBuildRestConfigFromKubeconfig(t *testing.T) {
	kubeconfigPath := writeTempKubeconfig(t)
	t.Setenv("KUBECONFIG", kubeconfigPath)

	tests := []struct {
		name       string
		context    string
		wantServer string
	}{
		{
			name:       "empty context uses current-context",
			context:    "",
			wantServer: "https://server-a.example.com",
		},
		{
			name:       "explicit context overrides current-context",
			context:    "ctx-b",
			wantServer: "https://server-b.example.com",
		},
		{
			name:       "explicit current-context still resolves",
			context:    "ctx-a",
			wantServer: "https://server-a.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := buildRestConfigFromKubeconfig(tt.context)
			if err != nil {
				t.Fatalf("buildRestConfigFromKubeconfig(%q) returned error: %v", tt.context, err)
			}
			if cfg.Host != tt.wantServer {
				t.Errorf("buildRestConfigFromKubeconfig(%q) host = %q, want %q", tt.context, cfg.Host, tt.wantServer)
			}
		})
	}
}

func TestBuildRestConfigFromKubeconfigUnknownContext(t *testing.T) {
	kubeconfigPath := writeTempKubeconfig(t)
	t.Setenv("KUBECONFIG", kubeconfigPath)

	if _, err := buildRestConfigFromKubeconfig("does-not-exist"); err == nil {
		t.Fatalf("expected error for unknown context, got nil")
	}
}
