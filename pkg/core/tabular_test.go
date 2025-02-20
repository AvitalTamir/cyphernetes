package core

import (
	"reflect"
	"testing"
)

func TestDocumentToTabular(t *testing.T) {
	tests := []struct {
		name         string
		result       *QueryResult
		returnClause *ReturnClause
		want         *TabularResult
		wantErr      bool
	}{
		{
			name: "basic conversion",
			result: &QueryResult{
				Data: map[string]interface{}{
					"p": []interface{}{
						map[string]interface{}{
							"metadata": map[string]interface{}{
								"name": "pod1",
							},
							"status": map[string]interface{}{
								"phase": "Running",
							},
						},
						map[string]interface{}{
							"metadata": map[string]interface{}{
								"name": "pod2",
							},
							"status": map[string]interface{}{
								"phase": "Pending",
							},
						},
					},
				},
			},
			returnClause: &ReturnClause{
				Items: []*ReturnItem{
					{JsonPath: "p.metadata.name"},
					{JsonPath: "p.status.phase", Alias: "status"},
				},
			},
			want: &TabularResult{
				Columns: []string{"p.metadata.name", "status"},
				Rows: []map[string]interface{}{
					{
						"p.metadata.name": "pod1",
						"status":          "Running",
					},
					{
						"p.metadata.name": "pod2",
						"status":          "Pending",
					},
				},
				NodeMap: map[string][]int{
					"p": {0, 1},
				},
			},
		},
		{
			name: "multiple nodes",
			result: &QueryResult{
				Data: map[string]interface{}{
					"p": []interface{}{
						map[string]interface{}{
							"metadata": map[string]interface{}{
								"name": "pod1",
							},
						},
					},
					"s": []interface{}{
						map[string]interface{}{
							"metadata": map[string]interface{}{
								"name": "svc1",
							},
						},
					},
				},
			},
			returnClause: &ReturnClause{
				Items: []*ReturnItem{
					{JsonPath: "p.metadata.name", Alias: "pod"},
					{JsonPath: "s.metadata.name", Alias: "service"},
				},
			},
			want: &TabularResult{
				Columns: []string{"pod", "service"},
				Rows: []map[string]interface{}{
					{
						"pod": "pod1",
					},
					{
						"service": "svc1",
					},
				},
				NodeMap: map[string][]int{
					"p": {0},
					"s": {1},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DocumentToTabular(tt.result, tt.returnClause)
			if (err != nil) != tt.wantErr {
				t.Errorf("DocumentToTabular() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DocumentToTabular() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTabularToDocument(t *testing.T) {
	tests := []struct {
		name    string
		tabular *TabularResult
		want    *QueryResult
	}{
		{
			name: "basic conversion",
			tabular: &TabularResult{
				Columns: []string{"p.metadata.name", "status"},
				Rows: []map[string]interface{}{
					{
						"p.metadata.name": "pod1",
						"status":          "Running",
					},
					{
						"p.metadata.name": "pod2",
						"status":          "Pending",
					},
				},
				NodeMap: map[string][]int{
					"p": {0, 1},
				},
			},
			want: &QueryResult{
				Data: map[string]interface{}{
					"p": []interface{}{
						map[string]interface{}{
							"metadata": map[string]interface{}{
								"name": "pod1",
							},
						},
						map[string]interface{}{
							"metadata": map[string]interface{}{
								"name": "pod2",
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TabularToDocument(tt.tabular)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("TabularToDocument() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestApplyOrderBy(t *testing.T) {
	tests := []struct {
		name    string
		tabular *TabularResult
		orderBy []*OrderByItem
		want    *TabularResult
		wantErr bool
	}{
		{
			name: "sort by name ascending",
			tabular: &TabularResult{
				Columns: []string{"p.metadata.name", "status"},
				Rows: []map[string]interface{}{
					{
						"p.metadata.name": "pod2",
						"status":          "Pending",
					},
					{
						"p.metadata.name": "pod1",
						"status":          "Running",
					},
				},
				NodeMap: map[string][]int{
					"p": {0, 1},
				},
			},
			orderBy: []*OrderByItem{
				{JsonPath: "p.metadata.name", Desc: false},
			},
			want: &TabularResult{
				Columns: []string{"p.metadata.name", "status"},
				Rows: []map[string]interface{}{
					{
						"p.metadata.name": "pod1",
						"status":          "Running",
					},
					{
						"p.metadata.name": "pod2",
						"status":          "Pending",
					},
				},
				NodeMap: map[string][]int{
					"p": {0, 1},
				},
			},
		},
		{
			name: "sort by name descending",
			tabular: &TabularResult{
				Columns: []string{"p.metadata.name", "status"},
				Rows: []map[string]interface{}{
					{
						"p.metadata.name": "pod1",
						"status":          "Running",
					},
					{
						"p.metadata.name": "pod2",
						"status":          "Pending",
					},
				},
				NodeMap: map[string][]int{
					"p": {0, 1},
				},
			},
			orderBy: []*OrderByItem{
				{JsonPath: "p.metadata.name", Desc: true},
			},
			want: &TabularResult{
				Columns: []string{"p.metadata.name", "status"},
				Rows: []map[string]interface{}{
					{
						"p.metadata.name": "pod2",
						"status":          "Pending",
					},
					{
						"p.metadata.name": "pod1",
						"status":          "Running",
					},
				},
				NodeMap: map[string][]int{
					"p": {0, 1},
				},
			},
		},
		{
			name: "sort by multiple fields",
			tabular: &TabularResult{
				Columns: []string{"p.metadata.name", "status"},
				Rows: []map[string]interface{}{
					{
						"p.metadata.name": "pod2",
						"status":          "Running",
					},
					{
						"p.metadata.name": "pod1",
						"status":          "Running",
					},
					{
						"p.metadata.name": "pod3",
						"status":          "Pending",
					},
				},
				NodeMap: map[string][]int{
					"p": {0, 1, 2},
				},
			},
			orderBy: []*OrderByItem{
				{JsonPath: "status", Desc: false},
				{JsonPath: "p.metadata.name", Desc: false},
			},
			want: &TabularResult{
				Columns: []string{"p.metadata.name", "status"},
				Rows: []map[string]interface{}{
					{
						"p.metadata.name": "pod3",
						"status":          "Pending",
					},
					{
						"p.metadata.name": "pod1",
						"status":          "Running",
					},
					{
						"p.metadata.name": "pod2",
						"status":          "Running",
					},
				},
				NodeMap: map[string][]int{
					"p": {0, 1, 2},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.tabular.ApplyOrderBy(tt.orderBy)
			if (err != nil) != tt.wantErr {
				t.Errorf("ApplyOrderBy() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(tt.tabular, tt.want) {
				t.Errorf("ApplyOrderBy() = %v, want %v", tt.tabular, tt.want)
			}
		})
	}
}

func TestApplyLimitAndSkip(t *testing.T) {
	tests := []struct {
		name    string
		tabular *TabularResult
		limit   int
		skip    int
		want    *TabularResult
	}{
		{
			name: "limit only",
			tabular: &TabularResult{
				Columns: []string{"p.metadata.name"},
				Rows: []map[string]interface{}{
					{"p.metadata.name": "pod1"},
					{"p.metadata.name": "pod2"},
					{"p.metadata.name": "pod3"},
				},
				NodeMap: map[string][]int{
					"p": {0, 1, 2},
				},
			},
			limit: 2,
			skip:  0,
			want: &TabularResult{
				Columns: []string{"p.metadata.name"},
				Rows: []map[string]interface{}{
					{"p.metadata.name": "pod1"},
					{"p.metadata.name": "pod2"},
				},
				NodeMap: map[string][]int{
					"p": {0, 1},
				},
			},
		},
		{
			name: "skip only",
			tabular: &TabularResult{
				Columns: []string{"p.metadata.name"},
				Rows: []map[string]interface{}{
					{"p.metadata.name": "pod1"},
					{"p.metadata.name": "pod2"},
					{"p.metadata.name": "pod3"},
				},
				NodeMap: map[string][]int{
					"p": {0, 1, 2},
				},
			},
			limit: 0,
			skip:  1,
			want: &TabularResult{
				Columns: []string{"p.metadata.name"},
				Rows: []map[string]interface{}{
					{"p.metadata.name": "pod2"},
					{"p.metadata.name": "pod3"},
				},
				NodeMap: map[string][]int{
					"p": {0, 1},
				},
			},
		},
		{
			name: "limit and skip",
			tabular: &TabularResult{
				Columns: []string{"p.metadata.name"},
				Rows: []map[string]interface{}{
					{"p.metadata.name": "pod1"},
					{"p.metadata.name": "pod2"},
					{"p.metadata.name": "pod3"},
					{"p.metadata.name": "pod4"},
				},
				NodeMap: map[string][]int{
					"p": {0, 1, 2, 3},
				},
			},
			limit: 2,
			skip:  1,
			want: &TabularResult{
				Columns: []string{"p.metadata.name"},
				Rows: []map[string]interface{}{
					{"p.metadata.name": "pod2"},
					{"p.metadata.name": "pod3"},
				},
				NodeMap: map[string][]int{
					"p": {0, 1},
				},
			},
		},
		{
			name: "skip all",
			tabular: &TabularResult{
				Columns: []string{"p.metadata.name"},
				Rows: []map[string]interface{}{
					{"p.metadata.name": "pod1"},
					{"p.metadata.name": "pod2"},
				},
				NodeMap: map[string][]int{
					"p": {0, 1},
				},
			},
			limit: 0,
			skip:  2,
			want: &TabularResult{
				Columns: []string{"p.metadata.name"},
				Rows:    nil,
				NodeMap: map[string][]int{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.tabular.ApplyLimitAndSkip(tt.limit, tt.skip)
			if !reflect.DeepEqual(tt.tabular, tt.want) {
				t.Errorf("ApplyLimitAndSkip() = %v, want %v", tt.tabular, tt.want)
			}
		})
	}
}

func TestFullRoundTrip(t *testing.T) {
	// Test a full round trip with all operations
	input := &QueryResult{
		Data: map[string]interface{}{
			"p": []interface{}{
				map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "pod3",
					},
					"status": map[string]interface{}{
						"phase": "Running",
					},
				},
				map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "pod1",
					},
					"status": map[string]interface{}{
						"phase": "Running",
					},
				},
				map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "pod2",
					},
					"status": map[string]interface{}{
						"phase": "Pending",
					},
				},
			},
		},
	}

	returnClause := &ReturnClause{
		Items: []*ReturnItem{
			{JsonPath: "p.metadata.name"},
			{JsonPath: "p.status.phase", Alias: "status"},
		},
		OrderBy: []*OrderByItem{
			{JsonPath: "p.metadata.name", Desc: false},
		},
		Limit: 2,
		Skip:  1,
	}

	// Convert to tabular
	tabular, err := DocumentToTabular(input, returnClause)
	if err != nil {
		t.Fatalf("DocumentToTabular() error = %v", err)
	}

	// Apply operations
	if err := tabular.ApplyOrderBy(returnClause.OrderBy); err != nil {
		t.Fatalf("ApplyOrderBy() error = %v", err)
	}
	tabular.ApplyLimitAndSkip(returnClause.Limit, returnClause.Skip)

	// Convert back to document
	got := TabularToDocument(tabular)

	// Expected result after sorting by name, skipping 1, and limiting to 2
	want := &QueryResult{
		Data: map[string]interface{}{
			"p": []interface{}{
				map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "pod2",
					},
				},
				map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "pod3",
					},
				},
			},
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("Full round trip = %v, want %v", got, want)
	}
}
