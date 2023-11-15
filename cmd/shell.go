package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/chzyer/readline"
	cobra "github.com/spf13/cobra"
)

var ShellCmd = &cobra.Command{
	Use:   "shell",
	Short: "Launch an interactive shell",
	Run:   runShell,
}

func runShell(cmd *cobra.Command, args []string) {
	rl, err := readline.New("> ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating readline: %s\n", err)
		return
	}
	defer rl.Close()
	rl.Config.HistoryFile = "/tmp/cyphernetes.history"

	fmt.Println("Cyphernetes Interactive Shell")
	fmt.Println("Type 'exit' or press Ctrl-D to exit")

	for {
		line, err := rl.Readline()
		if err != nil { // io.EOF, Ctrl-D
			break
		}

		input := strings.TrimSpace(line)
		if input == "exit" {
			break
		}

		// Process the input if not empty
		if input != "" {
			result, err := processQuery(input)
			if err != nil {
				fmt.Printf("Error: %s\n", err)
			} else {
				fmt.Println(result)
			}
		}
		// Add input to history
		rl.SaveHistory(input)
	}
}

func processQuery(query string) (string, error) {
	// Parse the query to get an AST.
	ast, err := ParseQuery(query)
	if err != nil {
		// Handle error.
		fmt.Println("Error parsing query: ", err)
		return "", err
	}

	// Execute the query against the Kubernetes API.
	executor, err := NewQueryExecutor()
	if err != nil {
		// Handle error.
		fmt.Println("Error creating query executor: ", err)
		return "", err
	}
	results, err := executor.Execute(ast)
	if err != nil {
		// Handle error.
		fmt.Println("Error executing query: ", err)
		return "", err
	}
	// Print the results as pretty JSON.
	json, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		// Handle error.
		fmt.Println("Error marshalling results: ", err)
		return "", err
	}
	return string(json), nil
}

func init() {
	rootCmd.AddCommand(ShellCmd)

	// Here you can define flags and configuration settings for the 'shell' subcommand if needed
}
