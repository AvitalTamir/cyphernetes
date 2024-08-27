package main

import (
	_ "embed"
	"os"
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
		{"getpo", 1, "List pods"},
		{"getdeploy", 1, "List deployments"},
		{"getsvc", 1, "List services"},
		{"getns", 1, "List namespaces"},
		{"getno", 1, "List nodes"},
		{"getpv", 1, "List persistent volumes"},
		{"getpvc", 1, "List persistent volume claims"},
		{"getevent", 1, "List warning events"},
		{"geting", 1, "List ingresses"},
		{"getcm", 1, "List config maps"},
		{"getsecret", 1, "List secrets"},
		{"getjob", 1, "List jobs"},
		{"getcronjob", 1, "List cron jobs"},
		{"podmon", 1, "Monitor pod resources"},
		{"expose", 2, "Expose a deployment as a service"},
		{"scale", 1, "Scale a deployment or statefulset"},
		{"exposepublic", 3, "Expose a deployment as a service and ingress"},
		{"deployexposure", 1, "Examine a deployment and its services and ingress"},
		{"createdeploy", 1, "Create a deployment"},
		{"countreplica", 1, "Count the number of desired vs available replicas for all deployments"},
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
	macrosToTest := []string{"getpo", "getdeploy", "getsvc"}
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

func TestAddMacroNoOverwrite(t *testing.T) {
	mm := NewMacroManager()
	macro1 := &Macro{Name: "test", Args: []string{"arg1"}, Statements: []string{"stmt1"}}
	macro2 := &Macro{Name: "test", Args: []string{"arg2"}, Statements: []string{"stmt2"}}

	mm.AddMacro(macro1, false)
	mm.AddMacro(macro2, false)

	if mm.Macros["test"] != macro1 {
		t.Errorf("Expected macro1 to remain, got %v", mm.Macros["test"])
	}
}

func TestExecuteMacroIncorrectArgCount(t *testing.T) {
	mm := NewMacroManager()
	macro := &Macro{Name: "test", Args: []string{"arg1"}, Statements: []string{"stmt1"}}
	mm.AddMacro(macro, false)

	_, err := mm.ExecuteMacro("test", []string{})
	if err == nil {
		t.Errorf("Expected error for incorrect argument count, got nil")
	}
}

func TestLoadMacrosFromFile(t *testing.T) {
	mm := NewMacroManager()

	// Test with non-existent file
	err := mm.LoadMacrosFromFile("non_existent_file.txt")
	if err == nil {
		t.Errorf("Expected error for non-existent file, got nil")
	}

	// Test with invalid macro definition
	tempFile, _ := os.CreateTemp("", "test_macro_*.txt")
	defer os.Remove(tempFile.Name())

	invalidContent := "123 invalid_macro\nInvalid content;\n"
	os.WriteFile(tempFile.Name(), []byte(invalidContent), 0644)

	err = mm.LoadMacrosFromFile(tempFile.Name())
	if err == nil {
		t.Errorf("Expected error for invalid macro definition, got nil")
	}

	// Test with valid macro but no statements
	validButEmptyContent := ":valid_macro arg1\n"
	os.WriteFile(tempFile.Name(), []byte(validButEmptyContent), 0644)

	err = mm.LoadMacrosFromFile(tempFile.Name())
	if err == nil {
		t.Errorf("Expected error for macro with no statements, got nil")
	}
}

func TestAddMacroWithOverwrite(t *testing.T) {
	mm := NewMacroManager()
	macro1 := &Macro{Name: "test", Args: []string{"arg1"}, Statements: []string{"stmt1"}}
	macro2 := &Macro{Name: "test", Args: []string{"arg2"}, Statements: []string{"stmt2"}}

	mm.AddMacro(macro1, false)
	mm.AddMacro(macro2, true)

	if mm.Macros["test"] != macro2 {
		t.Errorf("Expected macro2 to overwrite macro1, got %v", mm.Macros["test"])
	}
}

func TestExecuteMacroWithArgs(t *testing.T) {
	mm := NewMacroManager()
	macro := &Macro{Name: "test", Args: []string{"arg1"}, Statements: []string{"MATCH (n:$arg1) RETURN n"}}
	mm.AddMacro(macro, false)

	statements, err := mm.ExecuteMacro("test", []string{"Node"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if len(statements) != 1 || statements[0] != "MATCH (n:Node) RETURN n" {
		t.Errorf("Unexpected result: %v", statements)
	}
}

func TestInvalidMacroName(t *testing.T) {
	mm := NewMacroManager()
	macro := &Macro{Name: "invalid name", Args: []string{}, Statements: []string{"stmt1"}}

	err := mm.LoadMacrosFromString("test", ":!@# name\nstmt1\n")
	if err == nil {
		t.Errorf("Expected error for invalid macro name, got nil")
	}

	mm.AddMacro(macro, false)
	if _, exists := mm.Macros["!@#"]; exists {
		t.Errorf("Macro with invalid name should not be added")
	}
}

func TestLoadMacrosFromString(t *testing.T) {
	mm := NewMacroManager()
	macroString := `:macro1 arg1
stmt1
:macro2 arg1 arg2
stmt2
stmt2
stmt2`
	err := mm.LoadMacrosFromString("test", macroString)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(mm.Macros) != 2 {
		t.Errorf("Expected 2 macros, got %d", len(mm.Macros))
	}

	if macro, exists := mm.Macros["macro1"]; !exists || len(macro.Args) != 1 || len(macro.Statements) != 1 {
		t.Errorf("Unexpected macro1: %v, expected 1 arg and 1 statement, got %d args and %d statements. statements: %v", macro, len(macro.Args), len(macro.Statements), macro.Statements)
	}

	if macro, exists := mm.Macros["macro2"]; !exists || len(macro.Args) != 2 || len(macro.Statements) != 1 {
		t.Errorf("Unexpected macro2: %v, expected 2 args and 2 statements, got %d args and %d statements. statements: %v", macro, len(macro.Args), len(macro.Statements), macro.Statements)
	}
}

func TestExecuteNonExistentMacro(t *testing.T) {
	mm := NewMacroManager()
	_, err := mm.ExecuteMacro("non_existent", []string{})
	if err == nil {
		t.Errorf("Expected error for non-existent macro, got nil")
	}
}
