package main

import (
	_ "embed"
	"strings"
	"testing"
)

//go:embed default_macros.txt
var defaultMacrosContent string

func TestLoadDefaultMacros(t *testing.T) {
	// Create a new MacroManager
	mm := NewMacroManager()

	// Load the macros from the actual default_macros.txt file
	err := mm.loadMacros("default_macros.txt", strings.NewReader(defaultMacrosContent))
	if err != nil {
		t.Fatalf("Failed to load default macros: %v", err)
	}

	// Define expected macros (update this list based on your actual default_macros.txt content)
	expectedMacros := []struct {
		name        string
		statements  int
		description string
	}{
		{"po", 1, "List pods"},
		{"deploy", 1, "List deployments"},
		{"svc", 1, "List services"},
		{"ns", 1, "List namespaces"},
		{"no", 1, "List nodes"},
		{"pv", 1, "List persistent volumes"},
		{"pvc", 1, "List persistent volume claims"},
		{"event", 1, "List warning events"},
		{"podmon", 1, "Monitor pod resources"},
		{"ing", 1, "List ingresses"},
		{"cm", 1, "List config maps"},
		{"secret", 1, "List secrets"},
		{"job", 1, "List jobs"},
		{"cronjob", 1, "List cron jobs"},
		{"expose", 2, "Expose a deployment as a service"},
	}

	// Check if the expected macros were loaded correctly
	for _, expected := range expectedMacros {
		macro, exists := mm.Macros[expected.name]
		if !exists {
			t.Errorf("Expected macro '%s' not found", expected.name)
			continue
		}

		if len(macro.Statements) != expected.statements {
			t.Errorf("Macro '%s' has %d statements, expected %d", expected.name, len(macro.Statements), expected.statements)
		}

		if macro.Description != expected.description {
			t.Errorf("Macro '%s' has description '%s', expected '%s'", expected.name, macro.Description, expected.description)
		}
	}

	// Test executing a few loaded macros
	macrosToTest := []string{"po", "deploy", "svc"}
	for _, macroName := range macrosToTest {
		statements, err := mm.ExecuteMacro(macroName, []string{})
		if err != nil {
			t.Fatalf("Failed to execute '%s' macro: %v", macroName, err)
		}

		if len(statements) != 1 {
			t.Fatalf("Expected 1 statement from '%s' macro, got %d", macroName, len(statements))
		}

		// Check if the statement is not empty
		if strings.TrimSpace(statements[0]) == "" {
			t.Errorf("Macro '%s' returned an empty statement", macroName)
		}

		// You can add more specific checks for each macro's content if needed
	}
}
