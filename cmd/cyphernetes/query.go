package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/avitaltamir/cyphernetes/pkg/core"
	"github.com/spf13/cobra"
)

var (
	parseQuery       = core.ParseQuery
	newQueryExecutor = core.NewQueryExecutor
	executeMethod    = (*core.QueryExecutor).Execute
)

var queryCmd = &cobra.Command{
	Use:   "query [Cypher-inspired query]",
	Short: "Execute a Cypher-inspired query against Kubernetes",
	Long:  `Use the 'query' subcommand to execute a single Cypher-inspired query against your Kubernetes resources.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		runQuery(args, os.Stdout)
	},
}

func runQuery(args []string, w io.Writer) {
	// Parse the query to get an AST.
	ast, err := parseQuery(args[0])
	if err != nil {
		fmt.Fprintln(w, "Error parsing query: ", err)
		return
	}

	// Execute the query against the Kubernetes API.
	executor, err := newQueryExecutor()
	if err != nil {
		fmt.Fprintln(w, "Error creating query executor: ", err)
		return
	}
	results, err := executeMethod(executor, ast)
	if err != nil {
		fmt.Fprintln(w, "Error executing query: ", err)
		return
	}

	// Print the results as pretty JSON.
	json, err := json.MarshalIndent(results.Data, "", "  ")
	if err != nil {
		fmt.Fprintln(w, "Error marshalling results: ", err)
		return
	}
	if !disableColorJsonOutput {
		json = []byte(colorizeJson(string(json)))
	}

	if string(json) != "{}" {
		fmt.Fprintln(w, string(json))
	}
}

func init() {
	rootCmd.AddCommand(queryCmd)
	queryCmd.PersistentFlags().BoolVarP(&disableColorJsonOutput, "raw-output", "r", false, "Disable colorized JSON output")
}
