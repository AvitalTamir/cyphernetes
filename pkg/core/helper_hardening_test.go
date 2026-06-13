package core

import (
	"encoding/json"
	"reflect"
	"strings"
	"sync"
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestCacheExecutorAndMultiContextHelpers(t *testing.T) {
	provider := newHardeningProvider()

	oldOnce := once
	oldExecutor := executorInstance
	oldContextExecutors := contextExecutors
	oldGVRCache := GvrCache
	oldResourceSpecs := ResourceSpecs
	oldCleanOutput := CleanOutput
	oldRelationships := relationships
	oldPotentialKindsCache := potentialKindsCache
	defer func() {
		once = oldOnce
		executorInstance = oldExecutor
		contextExecutors = oldContextExecutors
		GvrCache = oldGVRCache
		ResourceSpecs = oldResourceSpecs
		CleanOutput = oldCleanOutput
		relationships = oldRelationships
		potentialKindsCache = oldPotentialKindsCache
	}()

	once = sync.Once{}
	executorInstance = nil
	contextExecutors = nil
	GvrCache = nil
	ResourceSpecs = nil
	CleanOutput = true

	if err := InitGVRCache(provider); err != nil {
		t.Fatal(err)
	}
	if GvrCache == nil {
		t.Fatal("expected InitGVRCache to initialize cache")
	}
	if err := InitResourceSpecs(provider); err != nil {
		t.Fatal(err)
	}
	if len(ResourceSpecs) == 0 {
		t.Fatal("expected resource specs to be loaded")
	}

	executor := GetQueryExecutorInstance(provider)
	if executor == nil {
		t.Fatal("expected singleton executor")
	}
	if executor.Provider() != provider {
		t.Fatal("singleton executor returned wrong provider")
	}
	specs, err := executor.GetOpenAPIResourceSpecs()
	if err != nil {
		t.Fatal(err)
	}
	if specs["io.k8s.api.core.v1.Pod"][0] != "metadata" {
		t.Fatalf("unexpected specs: %#v", specs)
	}

	contextExecutor, err := GetContextQueryExecutor("west")
	if err != nil {
		t.Fatal(err)
	}
	if again, err := GetContextQueryExecutor("west"); err != nil || again != contextExecutor {
		t.Fatalf("expected cached context executor, got %#v err %v", again, err)
	}

	ast := &Expression{
		Contexts: []string{"east", "west"},
		Clauses: []Clause{
			&MatchClause{
				Nodes: []*NodePattern{nodePattern("p", "Pod", &Properties{PropertyList: []*Property{{Key: "app", Value: "a"}}})},
			},
			&ReturnClause{Items: []*ReturnItem{{JsonPath: "p.metadata.name", Alias: "podName"}}},
		},
	}
	result, err := ExecuteMultiContextQuery(ast, "default")
	if err != nil {
		t.Fatal(err)
	}
	assertFirstValue(t, result, "east_p", "podName", "pod-a")
	assertFirstValue(t, result, "west_p", "podName", "pod-a")
}

func TestPrefixVariablesCoversAllClauseTypes(t *testing.T) {
	ast := &Expression{
		Contexts: []string{"ctx"},
		Clauses: []Clause{
			&MatchClause{
				Nodes: []*NodePattern{
					nodePattern("p", "Pod", nil),
					nodePattern("s", "Service", nil),
				},
				Relationships: []*Relationship{{
					Direction: Right,
					LeftNode:  nodePattern("p", "Pod", nil),
					RightNode: nodePattern("s", "Service", nil),
				}},
				ExtraFilters: []*Filter{{
					Type:         "KeyValuePair",
					KeyValuePair: &KeyValuePair{Key: "p.metadata.name", Value: "pod-a", Operator: "="},
				}},
			},
			&ReturnClause{Items: []*ReturnItem{{JsonPath: "p.metadata.name", Alias: "podName"}}},
			&SetClause{KeyValuePairs: []*KeyValuePair{{Key: "p.metadata.labels.team", Value: "core"}}},
			&DeleteClause{NodeIds: []string{"p"}},
			&CreateClause{
				Nodes: []*NodePattern{nodePattern("n", "Service", nil)},
				Relationships: []*Relationship{{
					Direction: Right,
					LeftNode:  nodePattern("n", "Service", nil),
					RightNode: nodePattern("p", "Pod", nil),
				}},
			},
		},
	}

	modified := prefixVariables(ast, "ctx")
	match := modified.Clauses[0].(*MatchClause)
	if match.Nodes[0].ResourceProperties.Name != "ctx_p" ||
		match.Relationships[0].LeftNode.ResourceProperties.Name != "ctx_p" ||
		match.ExtraFilters[0].KeyValuePair.Key != "ctx_p.metadata.name" {
		t.Fatalf("match clause was not prefixed: %#v", match)
	}
	if modified.Clauses[1].(*ReturnClause).Items[0].JsonPath != "ctx_p.metadata.name" {
		t.Fatalf("return clause was not prefixed: %#v", modified.Clauses[1])
	}
	if modified.Clauses[2].(*SetClause).KeyValuePairs[0].Key != "ctx_p.metadata.labels.team" {
		t.Fatalf("set clause was not prefixed: %#v", modified.Clauses[2])
	}
	if modified.Clauses[3].(*DeleteClause).NodeIds[0] != "ctx_p" {
		t.Fatalf("delete clause was not prefixed: %#v", modified.Clauses[3])
	}
	create := modified.Clauses[4].(*CreateClause)
	if create.Nodes[0].ResourceProperties.Name != "ctx_n" ||
		create.Relationships[0].RightNode.ResourceProperties.Name != "ctx_p" {
		t.Fatalf("create clause was not prefixed: %#v", create)
	}
}

func TestPatchJsonPathAndSetHelpers(t *testing.T) {
	patches := createCompatiblePatch([]string{"metadata", "labels", "app.kubernetes.io/name"}, "api")
	if patchPath(t, patches, 1) != "/metadata/labels/app.kubernetes.io~1name" {
		t.Fatalf("unexpected dotted label patch: %#v", patches)
	}

	patches = createCompatiblePatch([]string{"metadata", "annotations", "team"}, "core")
	if patchPath(t, patches, 1) != "/metadata/annotations/team" {
		t.Fatalf("unexpected annotation patch: %#v", patches)
	}

	patches = createCompatiblePatch([]string{"spec", "template", "spec", "containers[0]", "resources", "limits", "cpu"}, "500m")
	if patchPath(t, patches, 1) != "/spec/template/spec/containers/0/resources/limits/cpu" {
		t.Fatalf("unexpected container patch: %#v", patches)
	}

	patches = createCompatiblePatch([]string{"spec", "replicas"}, 3)
	if patchPath(t, patches, 0) != "/spec/replicas" {
		t.Fatalf("unexpected regular patch: %#v", patches)
	}

	resource := testPod("pod-a", "default", "a", 1)
	resource["metadata"].(map[string]interface{})["labels"].(map[string]interface{})["app.kubernetes.io/name"] = "api"
	if parts := splitEscapedPath("metadata.labels.app\\.kubernetes\\.io/name"); !reflect.DeepEqual(parts, []string{"metadata", "labels", "app.kubernetes.io/name"}) {
		t.Fatalf("unexpected escaped split: %#v", parts)
	}
	if value, err := JsonPathCompileAndLookup(resource, "$.metadata.labels.app\\.kubernetes\\.io/name"); err != nil || value != "api" {
		t.Fatalf("escaped jsonpath lookup = %v, %v", value, err)
	}

	containers := resource["spec"].(map[string]interface{})["containers"].([]interface{})
	containers = append(containers, map[string]interface{}{"name": "sidecar", "image": "redis"})
	resource["spec"].(map[string]interface{})["containers"] = containers
	if !evaluateWildcardPath(resource, "$.spec.containers[*].image", "redis", "=") {
		t.Fatal("expected wildcard path to match sidecar image")
	}
	if evaluateWildcardPath(resource, "$.spec.containers[*].image", "postgres", "=") {
		t.Fatal("unexpected wildcard path match")
	}
	if err := applyWildcardUpdate(resource, "$.spec.containers[*].image", "busybox"); err != nil {
		t.Fatal(err)
	}
	for _, item := range resource["spec"].(map[string]interface{})["containers"].([]interface{}) {
		if item.(map[string]interface{})["image"] != "busybox" {
			t.Fatalf("wildcard update missed container: %#v", resource)
		}
	}

	if err := setValueAtPath(resource, ".metadata.annotations.owner", "platform"); err != nil {
		t.Fatal(err)
	}
	metadata := resource["metadata"].(map[string]interface{})
	if metadata["annotations"].(map[string]interface{})["owner"] != "platform" {
		t.Fatalf("setValueAtPath did not update annotations: %#v", resource)
	}

	updateResultMap(resource, []string{"spec", "nodeSelector", "disk"}, "ssd")
	if resource["spec"].(map[string]interface{})["nodeSelector"].(map[string]interface{})["disk"] != "ssd" {
		t.Fatalf("updateResultMap did not create nested path: %#v", resource)
	}
	if err := setValueAtPath([]interface{}{}, ".metadata.name", "bad"); err == nil {
		t.Fatal("expected setValueAtPath to reject non-map data")
	}

	provider := newHardeningProvider()
	executor, _ := NewQueryExecutor(provider)
	patchJSON, _ := json.Marshal([]map[string]interface{}{{"op": "add", "path": "/metadata/labels/team", "value": "core"}})
	if err := executor.PatchK8sResource(resource, patchJSON, false); err != nil {
		t.Fatal(err)
	}
	if len(provider.patches) != 1 || !strings.Contains(provider.patches[0], "/metadata/labels/team") {
		t.Fatalf("patch was not recorded: %#v", provider.patches)
	}
}

func TestColumnarAndOrderingHelpers(t *testing.T) {
	cd := NewColumnarData()
	cd.AddRow(map[string]interface{}{"name": "pod-b", "score": 2}, "p", 1)
	cd.AddRow(map[string]interface{}{"name": "pod-a", "score": 1}, "p", 0)
	cd.AddRow(map[string]interface{}{"name": "svc-a", "score": 9}, "s", 0)

	if patterns := cd.GetPatternMatches(); len(patterns) != 2 {
		t.Fatalf("expected two pattern groups, got %#v", patterns)
	}
	nestedCD := NewColumnarData()
	nestedCD.AddRow(map[string]interface{}{"metadata": map[string]interface{}{"name": "nested"}}, "p", 0)
	if got := nestedCD.extractFieldValue("p.metadata.name", nestedCD.Rows[0], "p"); got != "nested" {
		t.Fatalf("unexpected extracted nested field: %v", got)
	}
	if got := navigateJSONPath(map[string]interface{}{"metadata": map[string]interface{}{"name": "pod-a"}}, "metadata.name"); got != "pod-a" {
		t.Fatalf("unexpected navigated value: %v", got)
	}
	if navigateJSONPath(map[string]interface{}{"metadata": "bad"}, "metadata.name") != nil {
		t.Fatal("expected invalid nested path to return nil")
	}

	if err := cd.OrderBy([]*OrderByItem{{Field: "name", Direction: "ASC"}}); err != nil {
		t.Fatal(err)
	}
	if got := cd.Rows[0][0]; got != "pod-a" {
		t.Fatalf("unexpected first sorted row: %#v", cd.Rows)
	}
	cd.Skip(1)
	if len(cd.Rows) != 1 {
		t.Fatalf("expected one row after skip, got %#v", cd.Rows)
	}
	cd.Limit(0)
	if len(cd.Rows) != 0 {
		t.Fatalf("expected empty rows after zero limit, got %#v", cd.Rows)
	}
	cd.Skip(5)

	aggregate := NewColumnarData()
	aggregate.AddRow(map[string]interface{}{"count": 3}, "aggregate", -1)
	if result := aggregate.ConvertToQueryResult(); result["aggregate"].(map[string]interface{})["count"] != 3 {
		t.Fatalf("unexpected aggregate conversion: %#v", result)
	}
	aggregate.Limit(5)

	for _, tt := range []struct {
		a, b interface{}
		want int
	}{
		{nil, nil, 0},
		{nil, "x", -1},
		{"b", "a", 1},
		{int64(1), int64(2), -1},
		{1.5, 1.0, 1},
		{false, true, -1},
		{"10", 2, 1},
	} {
		if got := compareOrderValues(tt.a, tt.b); got != tt.want {
			t.Fatalf("compareOrderValues(%v, %v) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestRelationshipStateAndComparisonHelpers(t *testing.T) {
	provider := newHardeningProvider()

	oldRules := relationshipRules
	relationshipRules = append([]RelationshipRule(nil), relationshipRules...)
	defer func() { relationshipRules = oldRules }()

	rules := findRelationshipRulesBetweenKinds("services", "pods")
	if len(rules) == 0 || rules[0].Relationship != ServiceExposePod {
		t.Fatalf("expected service/pod rule first, got %#v", rules)
	}

	ruleA := RelationshipRule{
		KindA:        "pod",
		KindB:        "service",
		Relationship: "A",
		MatchCriteria: []MatchCriterion{{
			FieldA:         "$.metadata.labels",
			FieldB:         "$.spec.selector",
			ComparisonType: ContainsAll,
		}},
	}
	ruleB := RelationshipRule{
		KindA:        "pods",
		KindB:        "services",
		Relationship: "B",
		MatchCriteria: []MatchCriterion{{
			FieldA:         "$.metadata.name",
			FieldB:         "$.metadata.name",
			ComparisonType: ExactMatch,
		}},
	}
	if !areRelatedRules(ruleA, ruleB, provider) {
		t.Fatal("expected singular and plural rules to relate")
	}
	consolidated := consolidateMatchingRules([]RelationshipRule{ruleA, ruleB, ruleB}, provider)
	if len(consolidated.MatchCriteria) != 2 {
		t.Fatalf("expected unique criteria to be consolidated, got %#v", consolidated.MatchCriteria)
	}
	if got := consolidateMatchingRules(nil, provider); got.Relationship != "" {
		t.Fatalf("expected empty consolidated rule, got %#v", got)
	}

	podGVR, _ := provider.FindGVR("pods")
	serviceGVR, _ := provider.FindGVR("services")
	gvrCache := map[string]schema.GroupVersionResource{
		"pods":     podGVR,
		"services": serviceGVR,
	}
	if idx, ok := findExistingRelationshipRule("pods", "services", gvrCache); !ok || idx < 0 {
		t.Fatalf("expected existing pod/service rule, got idx=%d ok=%t", idx, ok)
	}
	if _, ok := findExistingRelationshipRule("pods", "missing", gvrCache); ok {
		t.Fatal("did not expect relationship rule for missing GVR")
	}

	relationshipsMutex.Lock()
	oldRelationships := relationships
	relationships = map[string][]string{"pods": {"services"}}
	relationshipsMutex.Unlock()
	gotRelationships := GetRelationships()
	defer func() {
		relationshipsMutex.Lock()
		relationships = oldRelationships
		relationshipsMutex.Unlock()
	}()
	if !reflect.DeepEqual(gotRelationships["pods"], []string{"services"}) {
		t.Fatalf("unexpected relationships: %#v", gotRelationships)
	}

	if !containsResource([]map[string]interface{}{testPod("pod-a", "default", "a", 1)}, testPod("pod-a", "default", "b", 1)) {
		t.Fatal("expected containsResource to match by name and namespace")
	}
	if containsResource([]map[string]interface{}{{"kind": "Pod"}}, testPod("pod-a", "default", "a", 1)) {
		t.Fatal("did not expect malformed resource to match")
	}
}

func TestStateAggregateFilterAndUtilityHelpers(t *testing.T) {
	state := newExecutionState()
	pods := []map[string]interface{}{testPod("pod-a", "default", "a", 1)}
	state.cacheResources("pods:a", "p", pods)
	if !state.copyCachedResources("pods:a", "copy") {
		t.Fatal("expected cached resources to be copied")
	}
	if state.copyCachedResources("pods:missing", "copy") {
		t.Fatal("did not expect missing cache copy")
	}
	if got, err := requireResourceList(pods, "p"); err != nil || len(got) != 1 {
		t.Fatalf("unexpected resource list result: %v, %v", got, err)
	}
	if _, err := requireResourceList("bad", "p"); err == nil {
		t.Fatal("expected resource list type error")
	}

	filters := []*Filter{
		{Type: "SubMatch", SubMatch: &SubMatch{ReferenceNodeName: "p", IsNegated: true, Nodes: []*NodePattern{nodePattern("p", "Pod", nil)}}},
		nil,
		{Type: "KeyValuePair", KeyValuePair: &KeyValuePair{Key: "p.metadata.name", Value: "pod-a", Operator: "=", IsNegated: false}},
	}
	signature := filterSignature(filters)
	if !strings.Contains(signature, "kv:p.metadata.name:=:false") || !strings.Contains(signature, "sub:p:true:1:0") {
		t.Fatalf("unexpected filter signature: %s", signature)
	}

	subMatch := &SubMatch{
		IsNegated:         true,
		ReferenceNodeName: "p",
		Nodes:             []*NodePattern{nodePattern("p", "Pod", &Properties{PropertyList: []*Property{nil, {Key: "app", Value: "a"}}})},
		Relationships: []*Relationship{{
			Direction: Right,
			LeftNode:  nodePattern("p", "Pod", nil),
			RightNode: nodePattern("s", "Service", nil),
		}},
	}
	cloned := cloneSubMatch(subMatch)
	cloned.Nodes[0].ResourceProperties.Name = "changed"
	if subMatch.Nodes[0].ResourceProperties.Name != "p" || cloned.Relationships[0].LeftNode != cloned.Nodes[0] {
		t.Fatalf("clone did not preserve independent node graph: original=%#v clone=%#v", subMatch, cloned)
	}
	if cloneSubMatch(nil) != nil || cloneRelationship(nil, nil) != nil || cloneNodePattern(nil) != nil || cloneResourceProperties(nil) != nil {
		t.Fatal("nil clone helpers should return nil")
	}

	if result, filter, err := convertToComparableTypes(1, "2"); err != nil || result != float64(1) || filter != float64(2) {
		t.Fatalf("unexpected numeric conversion: %v %v %v", result, filter, err)
	}
	if result, filter, err := convertToComparableTypes("abc", true); err != nil || result != "abc" || filter != "true" {
		t.Fatalf("unexpected string fallback conversion: %v %v %v", result, filter, err)
	}
	if result, filter, err := convertToComparableTypes("abc", nil); err != nil || result != "abc" || filter != nil {
		t.Fatalf("unexpected nil conversion: %v %v %v", result, filter, err)
	}
	for _, v := range []interface{}{float32(1.5), int32(2), int64(3), "4"} {
		if _, err := toFloat64(v); err != nil {
			t.Fatalf("expected %T to convert: %v", v, err)
		}
	}
	if _, err := toFloat64(true); err == nil {
		t.Fatal("expected bool conversion to fail")
	}

	for _, tt := range []struct {
		resource interface{}
		filter   interface{}
		operator string
		want     bool
	}{
		{"a", "a", "EQUALS", true},
		{"a", "b", "!=", true},
		{3.0, 2.0, ">", true},
		{1.0, 2.0, "<", true},
		{2.0, 2.0, ">=", true},
		{2.0, 2.0, "<=", true},
		{"running", "run", "CONTAINS", true},
		{"pod-123", `pod-\d+`, "REGEX_COMPARE", true},
		{"pod", "[", "REGEX_COMPARE", false},
	} {
		if got := compareValues(tt.resource, tt.filter, tt.operator); got != tt.want {
			t.Fatalf("compareValues(%v, %v, %s) = %t", tt.resource, tt.filter, tt.operator, got)
		}
	}

	if !isResourceInList(testPod("pod-a", "default", "a", 1), pods) {
		t.Fatal("expected resource to be found in list")
	}
	if extractKindFromSchemaName("io.k8s.api.core.v1.Pod") != "Pod" || extractKindFromSchemaName("") != "" {
		t.Fatal("unexpected schema kind extraction")
	}
	if !contains([]string{"pod", "service"}, "service") || contains([]string{"pod"}, "deployment") {
		t.Fatal("contains returned unexpected result")
	}
	if doubled := Map([]int{1, 2}, func(item int, index int) int { return item*2 + index }); !reflect.DeepEqual(doubled, []int{2, 5}) {
		t.Fatalf("unexpected map result: %#v", doubled)
	}
	if value, ok := Find([]string{"pod", "service"}, func(item string) bool { return strings.HasPrefix(item, "ser") }); !ok || value != "service" {
		t.Fatalf("unexpected find hit: %s %t", value, ok)
	}
	if value, ok := Find([]string{"pod"}, func(item string) bool { return item == "service" }); ok || value != "" {
		t.Fatalf("unexpected find miss: %s %t", value, ok)
	}
	(&MatchClause{}).isClause()
	(&CreateClause{}).isClause()
	(&SetClause{}).isClause()
	(&DeleteClause{}).isClause()
	(&ReturnClause{}).isClause()
}

func TestLexerSettersAndQueryLiteralHelpers(t *testing.T) {
	lexer := NewLexer("MATCH")
	if lexer.Peek() == 0 {
		t.Fatal("expected lexer peek to see input")
	}
	lexer.SetParsingContexts(true)
	lexer.SetParsingJsonData(false)
	lexer.SetParsingJsonPath(true)
	if !lexer.inContexts || !lexer.inJsonData || !lexer.isInJsonPath {
		t.Fatalf("lexer flags not set: %#v", lexer)
	}

	if (&QueryExpandedError{ExpandedQuery: "MATCH (p:Pod)"}).Error() != "query expanded to: MATCH (p:Pod)" {
		t.Fatal("unexpected query expanded error string")
	}
	for _, tt := range []struct {
		value interface{}
		want  string
	}{
		{"hello \"world\"", `"hello \"world\""`},
		{true, "true"},
		{false, "false"},
		{nil, "null"},
		{3, "3"},
	} {
		if got := renderQueryLiteral(tt.value); got != tt.want {
			t.Fatalf("renderQueryLiteral(%#v) = %s, want %s", tt.value, got, tt.want)
		}
	}
}

func patchPath(t *testing.T, patches []interface{}, index int) string {
	t.Helper()
	patch, ok := patches[index].(map[string]interface{})
	if !ok {
		t.Fatalf("patch %d has unexpected type: %#v", index, patches[index])
	}
	path, ok := patch["path"].(string)
	if !ok {
		t.Fatalf("patch %d has no string path: %#v", index, patch)
	}
	return path
}
