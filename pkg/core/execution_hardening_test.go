package core

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/avitaltamir/cyphernetes/pkg/provider"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type hardeningProvider struct {
	mu        sync.Mutex
	resources map[string][]map[string]interface{}
	patches   []string
	deletes   []string
	creates   []string
}

func newHardeningProvider() *hardeningProvider {
	return &hardeningProvider{
		resources: map[string][]map[string]interface{}{
			"Pod": {
				testPod("pod-a", "default", "a", 2),
				testPod("pod-b", "default", "b", 1),
				testPod("pod-c", "default", "c", 3),
			},
			"Deployment": {
				testDeployment("deploy-a", "default", "a", 3),
				testDeployment("deploy-b", "default", "b", 1),
				testDeployment("deploy-c", "default", "c", 2),
			},
			"Service": {
				testService("svc-a", "default", "a"),
				testService("svc-b", "default", "b"),
			},
		},
	}
}

func (p *hardeningProvider) GetK8sResources(kind, fieldSelector, labelSelector, namespace string) (interface{}, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	source := p.resources[kind]
	var out []map[string]interface{}
	for _, resource := range source {
		metadata, metadataOK := resource["metadata"].(map[string]interface{})
		if namespace != "" && metadataOK && metadata["namespace"] != namespace {
			continue
		}
		if fieldSelector != "" && !matchesSelector(resource, fieldSelector) {
			continue
		}
		if labelSelector != "" && !matchesSelector(resource, labelSelector) {
			continue
		}
		out = append(out, cloneResourceMap(resource))
	}
	return out, nil
}

func (p *hardeningProvider) DeleteK8sResources(kind, name, namespace string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.deletes = append(p.deletes, fmt.Sprintf("%s/%s/%s", kind, namespace, name))
	return nil
}

func (p *hardeningProvider) CreateK8sResource(kind, name, namespace string, body interface{}) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.creates = append(p.creates, fmt.Sprintf("%s/%s/%s", kind, namespace, name))
	return nil
}

func (p *hardeningProvider) PatchK8sResource(kind, name, namespace string, patchJSON []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.patches = append(p.patches, fmt.Sprintf("%s/%s/%s:%s", kind, namespace, name, string(patchJSON)))
	return nil
}

func (p *hardeningProvider) FindGVR(kind string) (schema.GroupVersionResource, error) {
	switch strings.ToLower(kind) {
	case "pod", "pods":
		return schema.GroupVersionResource{Version: "v1", Resource: "pods"}, nil
	case "deployment", "deployments":
		return schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}, nil
	case "service", "services":
		return schema.GroupVersionResource{Version: "v1", Resource: "services"}, nil
	case "configmap", "configmaps":
		return schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}, nil
	case "secret", "secrets":
		return schema.GroupVersionResource{Version: "v1", Resource: "secrets"}, nil
	case "core.service":
		return schema.GroupVersionResource{Version: "v1", Resource: "services"}, nil
	default:
		return schema.GroupVersionResource{}, fmt.Errorf("unknown kind %s", kind)
	}
}

func (p *hardeningProvider) GetOpenAPIResourceSpecs() (map[string][]string, error) {
	return map[string][]string{
		"io.k8s.api.core.v1.Pod":        {"metadata", "spec"},
		"io.k8s.api.apps.v1.Deployment": {"metadata", "spec"},
		"io.k8s.api.core.v1.Service":    {"metadata", "spec"},
	}, nil
}

func (p *hardeningProvider) CreateProviderForContext(context string) (provider.Provider, error) {
	return p, nil
}

func (p *hardeningProvider) ToggleDryRun() {}

func TestExecutionUsesSelectorAwareCacheKeys(t *testing.T) {
	executor, _ := NewQueryExecutor(newHardeningProvider())
	result := executeTestQuery(t, executor, `MATCH (a:Pod {app: "a"}), (b:Pod {app: "b"}) RETURN a.metadata.name AS aName, b.metadata.name AS bName`)

	assertFirstValue(t, result, "a", "aName", "pod-a")
	assertFirstValue(t, result, "b", "bName", "pod-b")
}

func TestExecutionStateDoesNotLeakAfterError(t *testing.T) {
	executor, _ := NewQueryExecutor(newHardeningProvider())

	ast, err := ParseQuery(`MATCH (a:Pod {app: "a"}) SET missing.metadata.labels.foo = "bar"`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := executor.Execute(ast, "default"); err == nil {
		t.Fatal("expected first query to fail")
	}

	result := executeTestQuery(t, executor, `MATCH (b:Pod {app: "b"}) RETURN b.metadata.name AS bName`)
	assertFirstValue(t, result, "b", "bName", "pod-b")
}

func TestRepeatedAndConcurrentExecutionsAreIndependent(t *testing.T) {
	executor, _ := NewQueryExecutor(newHardeningProvider())
	queries := []string{
		`MATCH (p:Pod {app: "a"}) RETURN p.metadata.name AS name`,
		`MATCH (p:Pod {app: "b"}) RETURN p.metadata.name AS name`,
		`MATCH (p:Pod {app: "c"}) RETURN p.metadata.name AS name`,
	}

	for i, query := range queries {
		result := executeTestQuery(t, executor, query)
		assertFirstValue(t, result, "p", "name", fmt.Sprintf("pod-%c", 'a'+rune(i)))
	}

	var wg sync.WaitGroup
	errs := make(chan error, len(queries))
	for i, query := range queries {
		wg.Add(1)
		go func(i int, query string) {
			defer wg.Done()
			ast, err := ParseQuery(query)
			if err != nil {
				errs <- err
				return
			}
			result, err := executor.Execute(ast, "default")
			if err != nil {
				errs <- err
				return
			}
			got := firstValue(result, "p", "name")
			want := fmt.Sprintf("pod-%c", 'a'+rune(i))
			if got != want {
				errs <- fmt.Errorf("got %v, want %s", got, want)
			}
		}(i, query)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestKindlessRewriteUsesValidStringLiteralsForSet(t *testing.T) {
	oldMock := mockFindPotentialKinds
	mockFindPotentialKinds = func([]*Relationship) []string { return []string{"Pod"} }
	defer func() { mockFindPotentialKinds = oldMock }()

	executor, _ := NewQueryExecutor(newHardeningProvider())
	ast, err := ParseQuery(`MATCH (d:Deployment)->(x) SET d.metadata.labels.note = "hello \"world\"" RETURN d.metadata.name AS name`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := executor.rewriteQueryForKindlessNodes(ast); err != nil {
		t.Fatalf("rewrite should render parseable literals: %v", err)
	}
}

func TestRelationshipConsolidationDoesNotMutateGlobalRules(t *testing.T) {
	originalRules := relationshipRules
	relationshipRules = append([]RelationshipRule(nil), relationshipRules...)
	defer func() { relationshipRules = originalRules }()

	before := append([]MatchCriterion(nil), relationshipRules[0].MatchCriteria...)
	provider := newHardeningProvider()
	executor, _ := NewQueryExecutor(provider)
	state := newExecutionState()
	results := &QueryResult{Data: map[string]interface{}{}, Graph: Graph{}}
	match := &MatchClause{
		Nodes: []*NodePattern{
			nodePattern("p", "Pod", nil),
			nodePattern("s", "Service", nil),
		},
		Relationships: []*Relationship{{
			Direction: Right,
			LeftNode:  nodePattern("p", "Pod", nil),
			RightNode: nodePattern("s", "Service", nil),
		}},
	}

	_, err := executor.processRelationship(match.Relationships[0], match, results, map[string][]map[string]interface{}{}, state)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, relationshipRules[0].MatchCriteria) {
		t.Fatal("relationship consolidation mutated global rules")
	}
}

func TestSubMatchDoesNotMutateParsedAST(t *testing.T) {
	executor, _ := NewQueryExecutor(newHardeningProvider())
	ast, err := ParseQuery(`MATCH (p:Pod) WHERE (p)->(:Service) RETURN p.metadata.name AS name`)
	if err != nil {
		t.Fatal(err)
	}
	match := ast.Clauses[0].(*MatchClause)
	subMatch := match.ExtraFilters[0].SubMatch

	if _, err := executor.checkSubMatch(subMatch, "p", newExecutionState()); err != nil {
		t.Fatal(err)
	}
	if subMatch.Nodes[0].ResourceProperties.Name != "p" {
		t.Fatalf("submatch node name mutated to %s", subMatch.Nodes[0].ResourceProperties.Name)
	}
}

func TestColumnarOperationsDoNotTreatUnrelatedEqualSizedResultsAsPatternRows(t *testing.T) {
	executor, _ := NewQueryExecutor(newHardeningProvider())
	result := executeTestQuery(t, executor, `MATCH (p:Pod), (d:Deployment) RETURN p.metadata.name AS podName, d.metadata.name AS deploymentName ORDER BY podName ASC LIMIT 2`)

	totalRows := 0
	for _, rows := range result.Data {
		if rowSlice, ok := rows.([]interface{}); ok {
			totalRows += len(rowSlice)
		}
	}
	if totalRows != 2 {
		t.Fatalf("expected LIMIT to apply to independent rows, got %d total rows in %#v", totalRows, result.Data)
	}
}

func TestGraphDedupesNodes(t *testing.T) {
	executor, _ := NewQueryExecutor(newHardeningProvider())
	result := executeTestQuery(t, executor, `MATCH (p:Pod {app: "a"}) RETURN p, p.metadata.name AS name`)

	seen := map[string]bool{}
	for _, node := range result.Graph.Nodes {
		key := node.Kind + "/" + node.Namespace + "/" + node.Name
		if seen[key] {
			t.Fatalf("duplicate graph node %s in %#v", key, result.Graph.Nodes)
		}
		seen[key] = true
	}
}

func TestMalformedProviderResourceReturnsError(t *testing.T) {
	provider := newHardeningProvider()
	provider.resources["Pod"] = []map[string]interface{}{{"kind": "Pod"}}
	executor, _ := NewQueryExecutor(provider)
	ast, err := ParseQuery(`MATCH (p:Pod) RETURN p.metadata.name AS name`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = executor.Execute(ast, "default")
	if err == nil || !strings.Contains(err.Error(), "resources were not loaded") && !strings.Contains(err.Error(), "metadata") {
		t.Fatalf("expected metadata error, got %v", err)
	}
}

func TestSetDeleteAndCreateUseExecutionState(t *testing.T) {
	provider := newHardeningProvider()
	executor, _ := NewQueryExecutor(provider)

	executeTestQuery(t, executor, `MATCH (p:Pod {app: "a"}) SET p.metadata.labels.tier = "frontend" RETURN p.metadata.name AS name`)
	executeTestQuery(t, executor, `MATCH (p:Pod {app: "b"}) DELETE p`)
	executeTestQuery(t, executor, `CREATE (s:Service {"metadata":{"name":"created-svc"},"spec":{"selector":{"app":"created"}}})`)

	if len(provider.patches) != 1 {
		t.Fatalf("expected one patch, got %v", provider.patches)
	}
	if len(provider.deletes) != 1 {
		t.Fatalf("expected one delete, got %v", provider.deletes)
	}
	if len(provider.creates) != 1 {
		t.Fatalf("expected one create, got %v", provider.creates)
	}
}

func executeTestQuery(t *testing.T, executor *QueryExecutor, query string) QueryResult {
	t.Helper()
	ast, err := ParseQuery(query)
	if err != nil {
		t.Fatalf("parse query: %v", err)
	}
	result, err := executor.Execute(ast, "default")
	if err != nil {
		t.Fatalf("execute query: %v", err)
	}
	return result
}

func assertFirstValue(t *testing.T, result QueryResult, node, key string, want interface{}) {
	t.Helper()
	if got := firstValue(result, node, key); got != want {
		t.Fatalf("result[%s][0][%s] = %v, want %v; full result: %#v", node, key, got, want, result.Data)
	}
}

func firstValue(result QueryResult, node, key string) interface{} {
	rows := result.Data[node].([]interface{})
	return rows[0].(map[string]interface{})[key]
}

func matchesSelector(resource map[string]interface{}, selector string) bool {
	for _, part := range strings.Split(selector, ",") {
		key, value, ok := strings.Cut(part, "=")
		if !ok {
			return false
		}
		if key == "metadata.name" {
			metadata, ok := resource["metadata"].(map[string]interface{})
			if !ok {
				return false
			}
			if metadata["name"] != value {
				return false
			}
			continue
		}
		metadata, ok := resource["metadata"].(map[string]interface{})
		if !ok {
			return false
		}
		labels, _ := metadata["labels"].(map[string]interface{})
		if labels[key] != value {
			return false
		}
	}
	return true
}

func cloneResourceMap(resource map[string]interface{}) map[string]interface{} {
	data, _ := json.Marshal(resource)
	var cloned map[string]interface{}
	_ = json.Unmarshal(data, &cloned)
	return cloned
}

func testPod(name, namespace, app string, replicas int) map[string]interface{} {
	return map[string]interface{}{
		"kind": "Pod",
		"metadata": map[string]interface{}{
			"name":              name,
			"namespace":         namespace,
			"creationTimestamp": "2026-06-07T10:00:00Z",
			"labels": map[string]interface{}{
				"app": app,
			},
		},
		"status": map[string]interface{}{
			"phase": "Running",
		},
		"spec": map[string]interface{}{
			"replicas": replicas,
			"values":   []interface{}{app},
			"resources": map[string]interface{}{
				"requests": map[string]interface{}{
					"cpu":    "250m",
					"memory": "128Mi",
				},
			},
			"containers": []interface{}{
				map[string]interface{}{
					"name":  "main",
					"image": "nginx",
					"resources": map[string]interface{}{
						"requests": map[string]interface{}{
							"cpu":    "250m",
							"memory": "128Mi",
						},
					},
					"volumeMounts": []interface{}{
						map[string]interface{}{"name": "config"},
					},
				},
			},
		},
	}
}

func testDeployment(name, namespace, app string, replicas int) map[string]interface{} {
	return map[string]interface{}{
		"kind": "Deployment",
		"metadata": map[string]interface{}{
			"name":              name,
			"namespace":         namespace,
			"creationTimestamp": "2026-06-07T10:00:00Z",
			"labels": map[string]interface{}{
				"app": app,
			},
		},
		"spec": map[string]interface{}{
			"replicas": replicas,
			"selector": map[string]interface{}{
				"matchLabels": map[string]interface{}{"app": app},
			},
			"template": map[string]interface{}{
				"metadata": map[string]interface{}{
					"labels": map[string]interface{}{"app": app},
				},
				"spec": map[string]interface{}{
					"containers": []interface{}{
						map[string]interface{}{
							"name":  "main",
							"image": "nginx",
							"resources": map[string]interface{}{
								"requests": map[string]interface{}{
									"cpu":    "250m",
									"memory": "128Mi",
								},
							},
						},
					},
				},
			},
		},
	}
}

func testService(name, namespace, app string) map[string]interface{} {
	return map[string]interface{}{
		"kind": "Service",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": namespace,
			"labels": map[string]interface{}{
				"app": app,
			},
		},
		"spec": map[string]interface{}{
			"selector": map[string]interface{}{"app": app},
		},
	}
}

func nodePattern(name, kind string, properties *Properties) *NodePattern {
	return &NodePattern{ResourceProperties: &ResourceProperties{Name: name, Kind: kind, Properties: properties}}
}
