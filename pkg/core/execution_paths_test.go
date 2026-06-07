package core

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/avitaltamir/cyphernetes/pkg/provider"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type unknownClause struct{}

func (*unknownClause) isClause() {}

func TestExecuteCoversNamespaceFiltersAndWhereBranches(t *testing.T) {
	provider := newHardeningProvider()
	provider.resources["Pod"] = append(provider.resources["Pod"], testPod("pod-other", "other", "other", 4))
	executor, _ := NewQueryExecutor(provider)

	oldAllNamespaces := AllNamespaces
	AllNamespaces = true
	allNamespacesResult := executeTestQuery(t, executor, `MATCH (p:Pod) RETURN p.metadata.name AS name ORDER BY name ASC`)
	defer func() { AllNamespaces = oldAllNamespaces }()
	if got := len(allNamespacesResult.Data["p"].([]interface{})); got != 4 {
		t.Fatalf("expected all namespaces query to return 4 pods, got %d: %#v", got, allNamespacesResult.Data)
	}
	if AllNamespaces {
		t.Fatal("ExecuteSingleQuery should reset AllNamespaces after use")
	}

	namespaceResult := executeTestQuery(t, executor, `MATCH (p:Pod {namespace: "other"}) RETURN p.metadata.name AS name`)
	assertFirstValue(t, namespaceResult, "p", "name", "pod-other")

	whereResult := executeTestQuery(t, executor, `MATCH (p:Pod) WHERE p.spec.replicas > 1 RETURN p.metadata.name AS name ORDER BY name ASC`)
	if got := len(whereResult.Data["p"].([]interface{})); got != 2 {
		t.Fatalf("expected two pods after numeric WHERE filter, got %#v", whereResult.Data)
	}

	notResult := executeTestQuery(t, executor, `MATCH (p:Pod) WHERE NOT p.metadata.name = "pod-a" RETURN p.metadata.name AS name`)
	for _, row := range notResult.Data["p"].([]interface{}) {
		if row.(map[string]interface{})["name"] == "pod-a" {
			t.Fatalf("negated WHERE kept pod-a: %#v", notResult.Data)
		}
	}

	wildcardResult := executeTestQuery(t, executor, `MATCH (p:Pod) WHERE p.spec.containers[*].image = "nginx" RETURN p.metadata.name AS name`)
	if got := len(wildcardResult.Data["p"].([]interface{})); got != 3 {
		t.Fatalf("expected wildcard WHERE to match default namespace pods, got %#v", wildcardResult.Data)
	}

	ast, err := ParseQuery(`MATCH (p:Pod {name: "pod-a", app: "a"}) RETURN p.metadata.name AS name`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := executor.Execute(ast, "default"); err == nil || !strings.Contains(err.Error(), "name") {
		t.Fatalf("expected name selector combination error, got %v", err)
	}
}

func TestExecuteAggregatesAndReturnShapes(t *testing.T) {
	executor, _ := NewQueryExecutor(newHardeningProvider())

	countResult := executeTestQuery(t, executor, `MATCH (p:Pod) RETURN COUNT{p} AS totalPods`)
	if got := countResult.Data["aggregate"].(map[string]interface{})["totalPods"]; got != 3 {
		t.Fatalf("COUNT aggregate = %v, want 3", got)
	}

	replicasResult := executeTestQuery(t, executor, `MATCH (p:Pod) RETURN SUM{p.spec.replicas} AS totalReplicas`)
	if got := replicasResult.Data["aggregate"].(map[string]interface{})["totalReplicas"]; got != float64(6) {
		t.Fatalf("replica SUM = %v, want 6", got)
	}

	cpuResult := executeTestQuery(t, executor, `MATCH (p:Pod) RETURN SUM{p.spec.resources.requests.cpu} AS totalCPU`)
	if got := cpuResult.Data["aggregate"].(map[string]interface{})["totalCPU"]; got != "750m" {
		t.Fatalf("cpu SUM = %v, want 750m", got)
	}

	memoryResult := executeTestQuery(t, executor, `MATCH (p:Pod) RETURN SUM{p.spec.resources.requests.memory} AS totalMemory`)
	if got := memoryResult.Data["aggregate"].(map[string]interface{})["totalMemory"]; got != "384Mi" {
		t.Fatalf("memory SUM = %v, want 384Mi", got)
	}

	containerResult := executeTestQuery(t, executor, `MATCH (p:Pod) RETURN SUM{p.spec.containers[*].image} AS images`)
	images := containerResult.Data["aggregate"].(map[string]interface{})["images"].([]interface{})
	if len(images) != 3 {
		t.Fatalf("expected container wildcard SUM to combine three images, got %#v", images)
	}

	rootResult := executeTestQuery(t, executor, `MATCH (p:Pod {app: "a"}) RETURN p`)
	if row := rootResult.Data["p"].([]interface{})[0].(map[string]interface{}); row["$"] == nil {
		t.Fatalf("expected root return key, got %#v", row)
	}

	nestedResult := executeTestQuery(t, executor, `MATCH (p:Pod {app: "a"}) RETURN p.metadata.labels.app`)
	row := nestedResult.Data["p"].([]interface{})[0].(map[string]interface{})
	if row["metadata"].(map[string]interface{})["labels"].(map[string]interface{})["app"] != "a" {
		t.Fatalf("expected nested return map, got %#v", row)
	}

	ast, err := ParseQuery(`MATCH (p:Pod) RETURN SUM{p.spec.values[*]} AS values`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := executor.Execute(ast, "default"); err == nil || !strings.Contains(err.Error(), "unsupported type for SUM") {
		t.Fatalf("expected unsupported slice SUM error, got %v", err)
	}
}

func TestExecuteRelationshipCreateAndErrorBranches(t *testing.T) {
	provider := newHardeningProvider()
	executor, _ := NewQueryExecutor(provider)

	executeTestQuery(t, executor, `MATCH (d:Deployment {app: "a"}) CREATE (d)->(s:Service)`)
	if len(provider.creates) != 1 || !strings.HasPrefix(provider.creates[0], "Service/default/") {
		t.Fatalf("expected relationship create to create a service, got %#v", provider.creates)
	}

	ast, err := ParseQuery(`MATCH (d:Deployment {app: "a"}), (s:Service {app: "a"}) CREATE (d)->(s)`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := executor.Execute(ast, "default"); err == nil || !strings.Contains(err.Error(), "both nodes") {
		t.Fatalf("expected both-nodes-exist create error, got %v", err)
	}

	ast, err = ParseQuery(`CREATE (d:Deployment)->(s:Service)`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := executor.Execute(ast, "default"); err == nil || !strings.Contains(err.Error(), "neither node") {
		t.Fatalf("expected neither-node create error, got %v", err)
	}

	_, err = executor.ExecuteSingleQuery(&Expression{Clauses: []Clause{&DeleteClause{NodeIds: []string{"x__exp__0"}}}}, "default")
	if err != nil {
		t.Fatalf("expanded delete identifiers should be skipped, got %v", err)
	}

	if _, err := executor.ExecuteSingleQuery(nil, "default"); err == nil {
		t.Fatal("expected nil AST error")
	}
	if _, err := executor.ExecuteSingleQuery(&Expression{Clauses: []Clause{&unknownClause{}}}, "default"); err == nil || !strings.Contains(err.Error(), "unknown clause") {
		t.Fatalf("expected unknown clause error, got %v", err)
	}
}

func TestExecuteKindlessRewriteMergesExpandedResults(t *testing.T) {
	oldMock := mockFindPotentialKinds
	mockFindPotentialKinds = func([]*Relationship) []string { return []string{"Pod", "Deployment"} }
	defer func() { mockFindPotentialKinds = oldMock }()

	executor, _ := NewQueryExecutor(newHardeningProvider())
	result := executeTestQuery(t, executor, `MATCH (s:Service {app: "a"})->(x) RETURN x.metadata.name AS targetName, COUNT{x} AS targetCount`)

	rows, ok := result.Data["x"].([]interface{})
	if !ok || len(rows) != 2 {
		t.Fatalf("expected merged rows for kindless target, got %#v", result.Data)
	}
	names := map[interface{}]bool{}
	for _, row := range rows {
		names[row.(map[string]interface{})["targetName"]] = true
	}
	if !names["pod-a"] || !names["deploy-a"] {
		t.Fatalf("expected pod and deployment targets, got %#v", rows)
	}
	if got := result.Data["aggregate"].(map[string]interface{})["targetCount"]; got != 2 {
		t.Fatalf("merged aggregate count = %v, want 2", got)
	}
}

func TestRelationshipInitializationDiscoversAndCachesRules(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	provider := newHardeningProvider()

	oldRules := relationshipRules
	oldPotentialKindsCache := potentialKindsCache
	oldCleanOutput := CleanOutput
	relationshipRules = append([]RelationshipRule(nil), relationshipRules...)
	CleanOutput = true
	defer func() {
		relationshipRules = oldRules
		potentialKindsCache = oldPotentialKindsCache
		CleanOutput = oldCleanOutput
	}()

	specs := map[string][]string{
		"io.k8s.api.core.v1.Pod": {
			"spec.service.name",
			"spec.configMap.name",
			"spec.secretName",
			"spec.secretName",
		},
		"io.k8s.api.core.v1.Service":   {"metadata.name"},
		"io.k8s.api.core.v1.ConfigMap": {"metadata.name"},
		"io.k8s.api.core.v1.Secret":    {"metadata.name"},
	}
	before := len(relationshipRules)
	InitializeRelationships(specs, provider)
	if len(relationshipRules) <= before {
		t.Fatalf("expected discovered rules to be added")
	}

	var foundConfigMapRule bool
	var foundSecretRule bool
	for _, rule := range relationshipRules {
		if rule.KindA == "pods" && rule.KindB == "configmaps" {
			foundConfigMapRule = true
		}
		if rule.KindA == "pods" && rule.KindB == "secrets" {
			foundSecretRule = true
			if len(rule.MatchCriteria) < 2 {
				t.Fatalf("expected duplicate secret fields to append criteria, got %#v", rule.MatchCriteria)
			}
		}
	}
	if !foundConfigMapRule || !foundSecretRule {
		t.Fatalf("expected configmap and secret discovery, got %#v", relationshipRules[before:])
	}

	potentialKindsMutex.RLock()
	podPotentials := append([]string(nil), potentialKindsCache["core.pods"]...)
	potentialKindsMutex.RUnlock()
	if !contains(podPotentials, "core.services") || !contains(podPotentials, "core.configmaps") {
		t.Fatalf("expected potential kind cache to include discovered relationships, got %#v", podPotentials)
	}
}

func TestTemporalHandlerBranches(t *testing.T) {
	handler := NewTemporalHandler()
	base := "2026-06-07T10:00:00Z"

	exact, err := handler.EvaluateTemporalExpression(&TemporalExpression{Function: "datetime", Argument: base})
	if err != nil {
		t.Fatal(err)
	}
	if !exact.Equal(time.Date(2026, 6, 7, 10, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected datetime evaluation: %v", exact)
	}
	plus, err := handler.EvaluateTemporalExpression(&TemporalExpression{
		Function:  "datetime",
		Argument:  base,
		Operation: "+",
		RightExpr: &TemporalExpression{Function: "duration", Argument: "PT30M"},
	})
	if err != nil || !plus.Equal(exact.Add(30*time.Minute)) {
		t.Fatalf("unexpected datetime plus duration: %v %v", plus, err)
	}
	minus, err := handler.EvaluateTemporalExpression(&TemporalExpression{
		Function:  "duration",
		Argument:  "PT30M",
		Operation: "-",
		RightExpr: &TemporalExpression{Function: "datetime", Argument: base},
	})
	if err != nil || !minus.Equal(exact.Add(-30*time.Minute)) {
		t.Fatalf("unexpected duration minus datetime: %v %v", minus, err)
	}
	if _, err := handler.EvaluateTemporalExpression(&TemporalExpression{Function: "datetime", Argument: "bad"}); err == nil {
		t.Fatal("expected invalid datetime error")
	}
	if _, err := handler.EvaluateTemporalExpression(&TemporalExpression{Function: "datetime", Operation: "+"}); err == nil {
		t.Fatal("expected missing right expression error")
	}
	if _, err := handler.EvaluateTemporalExpression(&TemporalExpression{Function: "datetime", Argument: base, Operation: "*", RightExpr: &TemporalExpression{Function: "duration", Argument: "PT1H"}}); err == nil {
		t.Fatal("expected unsupported datetime operation error")
	}
	if _, err := handler.EvaluateTemporalExpression(&TemporalExpression{Function: "duration", Argument: "PT1H", Operation: "*", RightExpr: &TemporalExpression{Function: "datetime", Argument: base}}); err == nil {
		t.Fatal("expected unsupported duration operation error")
	}
	if _, err := handler.EvaluateTemporalExpression(&TemporalExpression{Function: "unknown"}); err == nil {
		t.Fatal("expected unsupported temporal function error")
	}

	for _, duration := range []string{"P1Y2M3DT4H5M6S", "PT30M"} {
		if _, err := handler.ParseISO8601Duration(duration); err != nil {
			t.Fatalf("expected duration %s to parse: %v", duration, err)
		}
	}
	for _, duration := range []string{"", "1H", "P", "PT", "PTY", "P1H", "PT1D", "PT1X", "PT1"} {
		if _, err := handler.ParseISO8601Duration(duration); err == nil {
			t.Fatalf("expected invalid duration %s to fail", duration)
		}
	}

	for _, tt := range []struct {
		operator string
		want     bool
	}{
		{"EQUALS", true},
		{"NOT_EQUALS", false},
		{"GREATER_THAN", false},
		{"LESS_THAN", false},
		{"GREATER_THAN_EQUALS", true},
		{"LESS_THAN_EQUALS", true},
	} {
		got, err := handler.CompareTemporalValues(exact, &TemporalExpression{Function: "datetime", Argument: base}, tt.operator)
		if err != nil || got != tt.want {
			t.Fatalf("CompareTemporalValues %s = %t, %v; want %t", tt.operator, got, err, tt.want)
		}
	}
	if _, err := handler.CompareTemporalValues(exact, &TemporalExpression{Function: "datetime", Argument: base}, "BAD"); err == nil {
		t.Fatal("expected bad temporal operator error")
	}
}

type ambiguousHardeningProvider struct {
	*hardeningProvider
}

func (p *ambiguousHardeningProvider) FindGVR(kind string) (schema.GroupVersionResource, error) {
	if kind == "service" {
		return schema.GroupVersionResource{}, fmt.Errorf("ambiguous resource kind: service\ncore.service\nnetworking.service")
	}
	return p.hardeningProvider.FindGVR(kind)
}

func (p *ambiguousHardeningProvider) CreateProviderForContext(context string) (provider.Provider, error) {
	return p, nil
}

func TestTryResolveGVRHandlesAmbiguousCoreResources(t *testing.T) {
	provider := &ambiguousHardeningProvider{hardeningProvider: newHardeningProvider()}
	executor, _ := NewQueryExecutor(provider)
	gvr, err := tryResolveGVR(provider, "service")
	if err != nil {
		t.Fatal(err)
	}
	if gvr.Resource != "services" {
		t.Fatalf("expected core service GVR, got %#v", gvr)
	}
	if _, err := tryResolveGVR(provider, "missing"); err == nil {
		t.Fatal("expected missing kind to fail")
	}
	key, err := executor.resourceFetchKey(nodePattern("p", "service", nil), "default", "", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(key, "services") {
		t.Fatalf("expected ambiguous kind cache key to resolve core service GVR, got %q", key)
	}
}
