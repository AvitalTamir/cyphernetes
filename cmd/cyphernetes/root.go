/*
Copyright © 2023 Avital Tamir <avital.osog@gmail.com>
*/
package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/avitaltamir/cyphernetes/pkg/parser"
	"github.com/spf13/cobra"
)

var (
	Version = "dev"
)

func getVersionInfo() string {
	return fmt.Sprintf("Version: %s\nGo Version: %s\n", Version, runtime.Version())
}

var rootCmd = &cobra.Command{
	Use:   "cyphernetes",
	Short: "Cyphernetes is a tool for querying Kubernetes resources",
	Long:  `Cyphernetes allows you to query Kubernetes resources using a Cypher-like query language.`,
	Run: func(cmd *cobra.Command, args []string) {
		versionFlag, _ := cmd.Flags().GetBool("version")
		if versionFlag {
			fmt.Print(getVersionInfo())
			os.Exit(0)
		}
		cmd.Help()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&parser.Namespace, "namespace", "n", "default", "The namespace to query against")
	rootCmd.PersistentFlags().StringVarP(&parser.LogLevel, "loglevel", "l", "info", "The log level to use (debug, info, warn, error, fatal, panic)")
	rootCmd.PersistentFlags().BoolVarP(&parser.AllNamespaces, "all-namespaces", "A", false, "Query all namespaces")
	rootCmd.PersistentFlags().BoolVar(&parser.NoColor, "no-color", false, "Disable colored output in shell and query results")
	rootCmd.Flags().BoolP("version", "v", false, "Show version and exit")
	rootCmd.MarkFlagsMutuallyExclusive("version", "v")

	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Print(getVersionInfo())
		},
	})

	rootCmd.AddCommand(WebCmd)
}
