package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"reflect"
	"testing"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

// DynamicOperator represents the structure of our CRD
type DynamicOperator struct {
	APIVersion string `yaml:"apiVersion" json:"apiVersion"`
	Kind       string `yaml:"kind" json:"kind"`
	Metadata   struct {
		Name string `yaml:"name" json:"name"`
	} `yaml:"metadata" json:"metadata"`
	Spec struct {
		ResourceKind string `yaml:"resourceKind" json:"resourceKind"`
		Namespace    string `yaml:"namespace" json:"namespace"`
		OnCreate     string `yaml:"onCreate,omitempty" json:"onCreate,omitempty"`
		OnUpdate     string `yaml:"onUpdate,omitempty" json:"onUpdate,omitempty"`
		OnDelete     string `yaml:"onDelete,omitempty" json:"onDelete,omitempty"`
	} `yaml:"spec" json:"spec"`
}

func TestRunCreate(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		onCreate string
		onUpdate string
		onDelete string
		want     DynamicOperator
	}{
		{
			name: "Default queries",
			args: []string{"test-operator"},
			want: DynamicOperator{
				APIVersion: "cyphernetes-operator.cyphernet.es/v1",
				Kind:       "DynamicOperator",
				Metadata: struct {
					Name string `yaml:"name" json:"name"`
				}{Name: "test-operator"},
				Spec: struct {
					ResourceKind string `yaml:"resourceKind" json:"resourceKind"`
					Namespace    string `yaml:"namespace" json:"namespace"`
					OnCreate     string `yaml:"onCreate,omitempty" json:"onCreate,omitempty"`
					OnUpdate     string `yaml:"onUpdate,omitempty" json:"onUpdate,omitempty"`
					OnDelete     string `yaml:"onDelete,omitempty" json:"onDelete,omitempty"`
				}{
					ResourceKind: "pods",
					Namespace:    "default",
					OnCreate:     "MATCH (p:Pods) RETURN p.metadata.name",
					OnUpdate:     "MATCH (p:Pods) RETURN p.metadata.name",
					OnDelete:     "MATCH (p:Pods) RETURN p.metadata.name",
				},
			},
		},
		{
			name:     "Custom onCreate query",
			args:     []string{"custom-operator"},
			onCreate: "MATCH (d:Deployment) CREATE (d)->(s:Service)",
			want: DynamicOperator{
				APIVersion: "cyphernetes-operator.cyphernet.es/v1",
				Kind:       "DynamicOperator",
				Metadata: struct {
					Name string `yaml:"name" json:"name"`
				}{Name: "custom-operator"},
				Spec: struct {
					ResourceKind string `yaml:"resourceKind" json:"resourceKind"`
					Namespace    string `yaml:"namespace" json:"namespace"`
					OnCreate     string `yaml:"onCreate,omitempty" json:"onCreate,omitempty"`
					OnUpdate     string `yaml:"onUpdate,omitempty" json:"onUpdate,omitempty"`
					OnDelete     string `yaml:"onDelete,omitempty" json:"onDelete,omitempty"`
				}{
					ResourceKind: "pods",
					Namespace:    "default",
					OnCreate:     "MATCH (d:Deployment) CREATE (d)->(s:Service)",
				},
			},
		},
		{
			name:     "Custom onUpdate and onDelete queries",
			args:     []string{"update-delete-operator"},
			onUpdate: "MATCH (d:Deployment) SET d.spec.replicas = 3",
			onDelete: "MATCH (d:Deployment)->(s:Service) DELETE s",
			want: DynamicOperator{
				APIVersion: "cyphernetes-operator.cyphernet.es/v1",
				Kind:       "DynamicOperator",
				Metadata: struct {
					Name string `yaml:"name" json:"name"`
				}{Name: "update-delete-operator"},
				Spec: struct {
					ResourceKind string `yaml:"resourceKind" json:"resourceKind"`
					Namespace    string `yaml:"namespace" json:"namespace"`
					OnCreate     string `yaml:"onCreate,omitempty" json:"onCreate,omitempty"`
					OnUpdate     string `yaml:"onUpdate,omitempty" json:"onUpdate,omitempty"`
					OnDelete     string `yaml:"onDelete,omitempty" json:"onDelete,omitempty"`
				}{
					ResourceKind: "pods",
					Namespace:    "default",
					OnUpdate:     "MATCH (d:Deployment) SET d.spec.replicas = 3",
					OnDelete:     "MATCH (d:Deployment)->(s:Service) DELETE s",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Redirect stdout to capture output
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			// Set up command and flags
			cmd := &cobra.Command{}
			cmd.Flags().StringVarP(&onCreate, "on-create", "c", "", "Query to run on resource creation")
			cmd.Flags().StringVarP(&onUpdate, "on-update", "u", "", "Query to run on resource update")
			cmd.Flags().StringVarP(&onDelete, "on-delete", "d", "", "Query to run on resource deletion")

			// Set flag values
			onCreate = tt.onCreate
			onUpdate = tt.onUpdate
			onDelete = tt.onDelete

			// Run the function
			runCreate(cmd, tt.args)

			// Restore stdout
			w.Close()
			os.Stdout = oldStdout

			// Read the output
			var buf bytes.Buffer
			io.Copy(&buf, r)
			output := buf.String()

			// Parse the YAML output
			var got DynamicOperator
			err := yaml.Unmarshal([]byte(output), &got)
			if err != nil {
				t.Fatalf("Failed to parse YAML output: %v", err)
			}

			// Compare the result
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("runCreate() output mismatch:\nGot:\n%s\nWant:\n%s", prettyPrint(got), prettyPrint(tt.want))
			}
		})
	}
}

func prettyPrint(v interface{}) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}
