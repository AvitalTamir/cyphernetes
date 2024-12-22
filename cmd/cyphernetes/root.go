/*
Copyright © 2023 Avital Tamir <avital.osog@gmail.com>
*/
package main

import (
	"os"

	"github.com/avitaltamir/cyphernetes/pkg/core"
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "cyphernetes",
	Short: "Cyphernetes is a tool for querying Kubernetes resources",
	Long:  `Cyphernetes allows you to query Kubernetes resources using a Cypher-like query language.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

// TestExecute is a helper function for testing the Execute function
func TestExecute(args []string) error {
	// Save the original os.Args
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	// Set up the new os.Args for testing
	os.Args = append([]string{"cmd"}, args...)

	// Create a new root command for testing
	cmd := &cobra.Command{Use: "test"}
	cmd.AddCommand(rootCmd)

	// Execute the command
	return cmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&core.Namespace, "namespace", "n", "default", "The namespace to query against")
	rootCmd.PersistentFlags().StringVarP(&core.LogLevel, "loglevel", "l", "info", "The log level to use (debug, info, warn, error, fatal, panic)")
	rootCmd.PersistentFlags().BoolVarP(&core.AllNamespaces, "all-namespaces", "A", false, "Query all namespaces")
	rootCmd.PersistentFlags().BoolVar(&core.NoColor, "no-color", false, "Disable colored output in shell and query results")

	// Add the web command
	rootCmd.AddCommand(WebCmd)
}
