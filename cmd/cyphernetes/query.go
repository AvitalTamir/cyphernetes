package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/avitaltamir/cyphernetes/pkg/core"
	"github.com/avitaltamir/cyphernetes/pkg/provider/apiserver"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
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
	PreRunE: func(cmd *cobra.Command, args []string) error {
		// Set CleanOutput to true before validating format
		core.CleanOutput = true

		// Validate format flag
		f := cmd.Flag("format").Value.String()
		if f != "yaml" && f != "json" {
			return fmt.Errorf("invalid value for --format: must be 'json' or 'yaml'")
		}
		// Initialize kubernetes before running the command
		return initializeKubernetes()
	},
	Run: func(cmd *cobra.Command, args []string) {
		provider, err := apiserver.NewAPIServerProviderWithOptions(&apiserver.APIServerProviderConfig{
			QuietMode: true,
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error creating provider: ", err)
			os.Exit(1)
		}
		executor = core.GetQueryExecutorInstance(provider)
		if executor == nil {
			os.Exit(1)
		}

		if err := core.InitResourceSpecs(executor.Provider()); err != nil {
			fmt.Printf("Error initializing resource specs: %v\n", err)
		}
		runQuery(args, os.Stdout)
	},
}

func runQuery(args []string, w io.Writer) {
	// Create the API server provider
	p, err := apiserver.NewAPIServerProviderWithOptions(&apiserver.APIServerProviderConfig{
		DryRun:    DryRun,
		QuietMode: true,
	})
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

	// Marshal data based on the output format
	var output []byte
	if core.OutputFormat == "yaml" {
		output, err = yaml.Marshal(results.Data)
	} else { // core.OutputFormat == "json"
		output, err = json.MarshalIndent(results.Data, "", "  ")
		if !returnRawJsonOutput {
			output = []byte(formatJson(string(output)))
		}
	}

	// Handle marshalling errors
	if err != nil {
		fmt.Fprintln(w, "Error marshalling results: ", err)
		return
	}

	if string(output) != "{}" {
		fmt.Fprintln(w, string(output))
	}
}

func init() {
	rootCmd.AddCommand(queryCmd)
	queryCmd.Flags().StringVar(&core.OutputFormat, "format", "json", "Output format (json or yaml)")
	queryCmd.PersistentFlags().BoolVarP(&returnRawJsonOutput, "raw-output", "r", false, "Disable JSON output formatting")
}
