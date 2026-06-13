package core

import (
	"sync"
	"testing"

	"github.com/avitaltamir/cyphernetes/pkg/provider"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// recordingProvider captures the dryRun flag passed to each mutating call so we
// can assert that Execute threads WithDryRun all the way down to the provider.
type recordingProvider struct {
	mu           sync.Mutex
	createDryRun []bool
}

func (p *recordingProvider) GetK8sResources(kind, fieldSelector, labelSelector, namespace string) (interface{}, error) {
	return map[string]interface{}{}, nil
}

func (p *recordingProvider) DeleteK8sResources(kind, name, namespace string, dryRun bool) error {
	return nil
}

func (p *recordingProvider) CreateK8sResource(kind, name, namespace string, body interface{}, dryRun bool) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.createDryRun = append(p.createDryRun, dryRun)
	return nil
}

func (p *recordingProvider) PatchK8sResource(kind, name, namespace string, patchJSON []byte, dryRun bool) error {
	return nil
}

func (p *recordingProvider) FindGVR(kind string) (schema.GroupVersionResource, error) {
	return schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}, nil
}

func (p *recordingProvider) GetOpenAPIResourceSpecs() (map[string][]string, error) {
	return map[string][]string{}, nil
}

func (p *recordingProvider) CreateProviderForContext(context string) (provider.Provider, error) {
	return p, nil
}

func (p *recordingProvider) lastCreateDryRun() (bool, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.createDryRun) == 0 {
		return false, false
	}
	return p.createDryRun[len(p.createDryRun)-1], true
}

// TestExecuteThreadsDryRunToProvider proves dry-run is a per-call property: the
// value passed via WithDryRun reaches the provider's mutating call, and the same
// executor yields different results on consecutive calls.
func TestExecuteThreadsDryRunToProvider(t *testing.T) {
	const createQuery = `CREATE (c:ConfigMap {"metadata": {"name": "x", "namespace": "default"}, "data": {"foo": "bar"}})`

	cases := []struct {
		name string
		opts []ExecuteOption
		want bool
	}{
		{name: "WithDryRun(true)", opts: []ExecuteOption{WithDryRun(true)}, want: true},
		{name: "WithDryRun(false)", opts: []ExecuteOption{WithDryRun(false)}, want: false},
		{name: "no option defaults to false", opts: nil, want: false},
	}

	rec := &recordingProvider{}
	executor, err := NewQueryExecutor(rec)
	if err != nil {
		t.Fatalf("NewQueryExecutor: %v", err)
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ast, err := ParseQuery(createQuery)
			if err != nil {
				t.Fatalf("ParseQuery: %v", err)
			}
			if _, err := executor.Execute(ast, "default", tc.opts...); err != nil {
				t.Fatalf("Execute: %v", err)
			}
			got, ok := rec.lastCreateDryRun()
			if !ok {
				t.Fatal("provider CreateK8sResource was never called")
			}
			if got != tc.want {
				t.Fatalf("provider received dryRun=%v, want %v", got, tc.want)
			}
		})
	}
}
