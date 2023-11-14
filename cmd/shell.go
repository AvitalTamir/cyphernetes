package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	cobra "github.com/spf13/cobra"
)

// ShellCommand is the Cobra command for the interactive shell
var ShellCmd = &cobra.Command{
	Use:   "shell",
	Short: "Launch an interactive shell",
	Run:   runShell,
}

func runShell(cmd *cobra.Command, args []string) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("Cyphernetes Interactive Shell")
	fmt.Println("Type 'exit' or press Ctrl-C to exit")

	for {
		fmt.Print("> ")
		input, err := reader.ReadString('\n')
		if err != nil {
			// Handle the error according to your needs
			break
		}

		input = strings.TrimSpace(input)
		if input == "exit" {
			break
		}

		// Process the input
		result, err := processQuery(input)
		if err != nil {
			fmt.Printf("Error: %s\n", err)
		} else {
			fmt.Println(result)
		}
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
