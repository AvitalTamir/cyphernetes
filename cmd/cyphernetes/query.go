package main

import (
	"fmt"

	"encoding/json"

	"github.com/avitaltamir/cyphernetes/pkg/parser"
	"github.com/spf13/cobra"
)

var queryCmd = &cobra.Command{
	Use:   "query [Cypher-inspired query]",
	Short: "Execute a Cypher-inspired query against Kubernetes",
	Long:  `Use the 'query' subcommand to execute a single Cypher-inspired query against your Kubernetes resources.`,
	Args:  cobra.ExactArgs(1), // This ensures that exactly one argument is provided
	Run: func(cmd *cobra.Command, args []string) {
		// Parse the query to get an AST.
		ast, err := parser.ParseQuery(args[0])
		if err != nil {
			// Handle error.
			fmt.Println("Error parsing query: ", err)
			return
		}

		// Execute the query against the Kubernetes API.
		executor, err := parser.NewQueryExecutor()
		if err != nil {
			// Handle error.
			fmt.Println("Error creating query executor: ", err)
			return
		}
		results, err := executor.Execute(ast)
		if err != nil {
			// Handle error.
			fmt.Println("Error executing query: ", err)
			return
		}

		// Print the results as pretty JSON.
		json, err := json.MarshalIndent(results, "", "  ")
		if err != nil {
			// Handle error.
			fmt.Println("Error marshalling results: ", err)
			return
		}
		if !disableColorJsonOutput {
			json = []byte(colorizeJson(string(json)))
		}

		if string(json) != "{}" {
			fmt.Println(string(json))
		}
	},
}

func init() {
	rootCmd.AddCommand(queryCmd)

	queryCmd.PersistentFlags().BoolVarP(&disableColorJsonOutput, "raw-output", "r", false, "Disable colorized JSON output")
}
