/*
Copyright © 2023 Avital Tamir <avital.osog@gmail.com>
*/
package main

import (
	"os"

	"github.com/avitaltamir/cyphernetes/pkg/parser"
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "cyphernetes",
	Short: "Cyphernetes is a CLI tool for managing Kubernetes resources using a Cypher-inspired query language",
	Long: `Cyphernetes allows users to interact with their Kubernetes resources 
           using a graph-like query language for complex operations.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) { },
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.cyphernetes.yaml)")

	// Add a namespace flag
	rootCmd.PersistentFlags().StringVarP(&parser.Namespace, "namespace", "n", "default", "The namespace to query against")
	rootCmd.PersistentFlags().StringVarP(&parser.LogLevel, "loglevel", "l", "info", "The log level to use (debug, info, warn, error, fatal, panic)")
	rootCmd.PersistentFlags().BoolVarP(&parser.AllNamespaces, "all-namespaces", "A", false, "Query all namespaces")
}
