/*
Copyright © 2023 Avital Tamir <avital.osog@gmail.com>
*/
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/avitaltamir/cyphernetes/pkg/core"
	"github.com/avitaltamir/cyphernetes/pkg/provider/apiserver"
	"github.com/spf13/cobra"
	"golang.org/x/term"
	"gopkg.in/yaml.v3"
)

var (
	Version             = "dev"
	DryRun              = false
	returnRawJsonOutput = false
)

var rootCmd = &cobra.Command{
	Use:   "kubectl-cypher [Cyphernetes query]",
	Short: "Execute Cyphernetes queries against Kubernetes as a kubectl plugin",
	Long:  `kubectl-cypher is a kubectl plugin that allows you to execute Cyphernetes queries against your Kubernetes resources.`,
	Args: func(cmd *cobra.Command, args []string) error {
		// Check version flag first
		versionFlag, _ := cmd.Flags().GetBool("version")
		if versionFlag {
			return nil
		}
		// Otherwise require exactly 1 argument
		return cobra.ExactArgs(1)(cmd, args)
	},
	PreRunE: func(cmd *cobra.Command, args []string) error {
		// Handle version flag
		versionFlag, _ := cmd.Flags().GetBool("version")
		if versionFlag {
			fmt.Print(getVersionInfo())
			os.Exit(0)
		}

		// Set CleanOutput to true before validating format
		core.CleanOutput = true

		// Validate format flag
		f := cmd.Flag("format").Value.String()
		if f != "yaml" && f != "json" {
			return fmt.Errorf("invalid value for --format: must be 'json' or 'yaml'")
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
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
	executor, err := core.NewQueryExecutor(p)
	if err != nil {
		fmt.Fprintln(w, "Error creating query executor: ", err)
		return
	}

	// Initialize resource specs
	if err := core.InitResourceSpecs(executor.Provider()); err != nil {
		fmt.Printf("Error initializing resource specs: %v\n", err)
	}

	// Get the query string
	queryStr := args[0]

	// Check if the input query consists only of comments and whitespace
	isOnlyComments := true
	potentialLines := strings.Split(queryStr, "\n")
	for _, potentialLine := range potentialLines {
		trimmedLine := strings.TrimSpace(potentialLine)
		if trimmedLine != "" && !strings.HasPrefix(trimmedLine, "//") {
			isOnlyComments = false
			break
		}
	}

	if isOnlyComments {
		// If only comments or empty, do nothing and exit cleanly.
		return
	}

	// Parse the query to get an AST
	ast, err := core.ParseQuery(queryStr)
	if err != nil {
		fmt.Fprintln(w, "Error parsing query: ", err)
		return
	}

	// Execute the query against the Kubernetes API.
	results, err := executor.Execute(ast, "")
	if err != nil {
		fmt.Fprintln(w, "Error executing query: ", err)
		return
	}

	// Marshal data based on the output format
	var output []byte
	if core.OutputFormat == "json" {
		output, err = json.MarshalIndent(results.Data, "", "  ")
		if err == nil && !returnRawJsonOutput && term.IsTerminal(int(os.Stdout.Fd())) {
			output = []byte(formatJson(string(output)))
		}
	} else {
		output, err = yaml.Marshal(results.Data)
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

func formatJson(jsonString string) string {
	var obj interface{}
	err := json.Unmarshal([]byte(jsonString), &obj)
	if err != nil {
		// Not a valid JSON object, likely an empty response.
		// We simply return the non-colorized payload.
		return jsonString
	}

	if core.NoColor {
		s, err := json.MarshalIndent(obj, "", "  ")
		if err != nil {
			fmt.Println("Error marshalling json: ", err)
			return jsonString
		}
		return string(s)
	}

	// For kubectl plugin, we'll keep it simple and not use colorjson
	// to avoid extra dependencies. Just return formatted JSON.
	s, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		fmt.Println("Error marshalling json: ", err)
		return jsonString
	}
	return string(s)
}

func getVersionInfo() string {
	return fmt.Sprintf(
		"kubectl-cypher %s\n"+
			"License: Apache 2.0\n"+
			"Source: https://github.com/avitaltamir/cyphernetes\n",
		Version,
	)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().StringVarP(&core.Namespace, "namespace", "n", "default", "The namespace to query against")
	rootCmd.PersistentFlags().BoolVarP(&core.AllNamespaces, "all-namespaces", "A", false, "Query all namespaces")
	rootCmd.PersistentFlags().BoolVar(&core.NoColor, "no-color", false, "Disable colored output")
	rootCmd.PersistentFlags().BoolVar(&DryRun, "dry-run", false, "Enable dry-run mode for all operations")

	// Command-specific flags
	rootCmd.Flags().StringVar(&core.OutputFormat, "format", "json", "Output format (json or yaml)")
	rootCmd.Flags().BoolVarP(&returnRawJsonOutput, "raw-output", "r", false, "Disable JSON output formatting")
	rootCmd.Flags().BoolP("version", "v", false, "Show version and exit")

}
