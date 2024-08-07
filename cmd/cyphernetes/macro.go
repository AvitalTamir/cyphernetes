package main

import (
	"bufio"
	_ "embed"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"
)

type Macro struct {
	Name        string
	Args        []string
	Statements  []string
	Description string
}

type MacroManager struct {
	Macros map[string]*Macro
}

func NewMacroManager() *MacroManager {
	return &MacroManager{
		Macros: make(map[string]*Macro),
	}
}

func (mm *MacroManager) AddMacro(macro *Macro, overwrite bool) {
	if _, exists := mm.Macros[macro.Name]; exists && !overwrite {
		return
	}
	mm.Macros[macro.Name] = macro
}

func (mm *MacroManager) ExecuteMacro(name string, args []string) ([]string, error) {
	macro, exists := mm.Macros[name]
	if !exists {
		return nil, fmt.Errorf("macro '%s' not found", name)
	}

	if len(args) != len(macro.Args) {
		return nil, fmt.Errorf("macro '%s' expects %d arguments, got %d", name, len(macro.Args), len(args))
	}

	statements := make([]string, len(macro.Statements))
	for i, stmt := range macro.Statements {
		for j, arg := range macro.Args {
			stmt = strings.ReplaceAll(stmt, "$"+arg, args[j])
		}
		statements[i] = stmt
	}

	return statements, nil
}

func (mm *MacroManager) LoadMacrosFromFile(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("failed to open macro file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var currentMacro *Macro
	var currentStatement strings.Builder
	lineNumber := 0

	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, ":") {
			// If we were processing a macro, add it to the manager
			if currentMacro != nil {
				if currentStatement.Len() > 0 {
					currentMacro.Statements = append(currentMacro.Statements, currentStatement.String())
				}
				if len(currentMacro.Statements) == 0 {
					return fmt.Errorf("macro '%s' has no statements (line %d)", currentMacro.Name, lineNumber)
				}
				mm.AddMacro(currentMacro, false)
			}

			// Start a new macro
			parts := strings.Fields(strings.TrimPrefix(line, ":"))
			if len(parts) == 0 {
				return fmt.Errorf("invalid macro definition at line %d: missing macro name", lineNumber)
			}
			name := parts[0]
			args := parts[1:]

			// Validate macro name
			if !isValidMacroName(name) {
				return fmt.Errorf("invalid macro name '%s' at line %d", name, lineNumber)
			}

			currentMacro = &Macro{Name: name, Args: args}
			currentStatement.Reset()
		} else if currentMacro == nil {
			return fmt.Errorf("statement found outside of macro definition at line %d", lineNumber)
		} else {
			// Add the line to the current statement
			if currentStatement.Len() > 0 {
				currentStatement.WriteString(" ")
			}
			currentStatement.WriteString(line)

			// If the line ends with a semicolon, add the statement to the macro
			if strings.HasSuffix(line, ";") {
				currentMacro.Statements = append(currentMacro.Statements, currentStatement.String())
				currentStatement.Reset()
			}
		}
	}

	// Add the last macro if there is one
	if currentMacro != nil {
		if currentStatement.Len() > 0 {
			currentMacro.Statements = append(currentMacro.Statements, currentStatement.String())
		}
		if len(currentMacro.Statements) == 0 {
			return fmt.Errorf("macro '%s' has no statements (line %d)", currentMacro.Name, lineNumber)
		}
		mm.AddMacro(currentMacro, false)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading macro file: %w", err)
	}

	return nil
}

func (mm *MacroManager) loadMacros(source string, reader io.Reader) error {
	scanner := bufio.NewScanner(reader)
	var currentMacro *Macro
	var currentStatement strings.Builder
	lineNumber := 0

	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, ":") {
			// If we were processing a macro, add it to the manager
			if currentMacro != nil {
				if currentStatement.Len() > 0 {
					currentMacro.Statements = append(currentMacro.Statements, currentStatement.String())
					currentStatement.Reset()
				}
				if len(currentMacro.Statements) == 0 {
					return fmt.Errorf("macro '%s' has no statements (line %d)", currentMacro.Name, lineNumber)
				}
				mm.AddMacro(currentMacro, source == "default_macros.txt")
			}

			// Start a new macro
			parts := strings.Fields(strings.TrimPrefix(line, ":"))
			if len(parts) == 0 {
				return fmt.Errorf("invalid macro definition at line %d: missing macro name", lineNumber)
			}
			name := parts[0]
			args := parts[1:]

			// Parse description if present
			description := ""
			for i, part := range parts {
				if strings.HasPrefix(part, "#") {
					description = strings.TrimSpace(strings.Join(parts[i:], " ")[1:])
					args = parts[1:i]
					break
				}
			}

			// Validate macro name
			if !isValidMacroName(name) {
				return fmt.Errorf("invalid macro name '%s' at line %d", name, lineNumber)
			}

			currentMacro = &Macro{Name: name, Args: args, Description: description}
		} else if currentMacro == nil {
			return fmt.Errorf("statement found outside of macro definition at line %d", lineNumber)
		} else {
			// Add the line to the current statement
			currentStatement.WriteString(line)
			if strings.HasSuffix(line, ";") {
				currentMacro.Statements = append(currentMacro.Statements, currentStatement.String())
				currentStatement.Reset()
			} else {
				currentStatement.WriteString(" ")
			}
		}
	}

	// Add the last macro if there is one
	if currentMacro != nil {
		if currentStatement.Len() > 0 {
			currentMacro.Statements = append(currentMacro.Statements, currentStatement.String())
		}
		if len(currentMacro.Statements) == 0 {
			return fmt.Errorf("macro '%s' has no statements (line %d)", currentMacro.Name, lineNumber)
		}
		mm.AddMacro(currentMacro, source == "default_macros.txt")
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading macro file: %w", err)
	}

	return nil
}

func isValidMacroName(name string) bool {
	if len(name) == 0 {
		return false
	}
	for i, r := range name {
		if i == 0 {
			if !unicode.IsLetter(r) && r != '_' {
				return false
			}
		} else {
			if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
				return false
			}
		}
	}
	return true
}

func (mm *MacroManager) LoadMacrosFromString(name, content string) error {
	reader := strings.NewReader(content)
	return mm.loadMacros(name, reader)
}
