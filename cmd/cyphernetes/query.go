package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/AvitalTamir/cyphernetes/pkg/core"
	"github.com/AvitalTamir/cyphernetes/pkg/provider/apiserver"
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
		provider, err := apiserver.NewAPIServerProvider()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error creating provider: ", err)
			os.Exit(1)
		}
		executor = core.GetQueryExecutorInstance(provider)
		if executor == nil {
			os.Exit(1)
		}
		core.CleanOutput = true
		if err := core.InitResourceSpecs(executor.Provider()); err != nil {
			fmt.Printf("Error initializing resource specs: %v\n", err)
		}
		runQuery(args, os.Stdout)
	},
}

func runQuery(args []string, w io.Writer) {
	// Create the API server provider
	p, err := apiserver.NewAPIServerProvider()
	if err != nil {
		fmt.Fprintln(w, "Error creating provider: ", err)
		return
	}

	// Create query executor with the provider
	executor, err := newQueryExecutor(p)
	if err != nil {
		fmt.Fprintln(w, "Error creating query executor: ", err)
		return
	}

	// Parse the query to get an AST
	ast, err := parseQuery(args[0])
	if err != nil {
		fmt.Fprintln(w, "Error parsing query: ", err)
		return
	}

	// Execute the query against the Kubernetes API.
	results, err := executeMethod(executor, ast, "")
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
	if !returnRawJsonOutput {
		json = []byte(formatJson(string(json)))
	}

	if string(json) != "{}" {
		fmt.Fprintln(w, string(json))
	}
}

func init() {
	rootCmd.AddCommand(queryCmd)
	queryCmd.PersistentFlags().BoolVarP(&returnRawJsonOutput, "raw-output", "r", false, "Disable JSON output formatting")
}
