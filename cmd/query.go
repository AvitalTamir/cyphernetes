package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// queryCmd represents the query command
var queryCmd = &cobra.Command{
	Use:   "query [Cypher-inspired query]",
	Short: "Execute a Cypher-inspired query against Kubernetes",
	Long:  `Use the 'query' subcommand to execute a single Cypher-inspired query against your Kubernetes resources.`,
	Args:  cobra.ExactArgs(1), // This ensures that exactly one argument is provided
	Run: func(cmd *cobra.Command, args []string) {
		// args[0] is the Cypher-inspired query string
		parsedResult, err := ParseQuery(args[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing query: %v\n", err)
			os.Exit(1)
		}
		// Now use the parsedResult to interact with Kubernetes
		fmt.Printf("Parsed result: %#v\n", parsedResult)
		// pretty print the parsed result

		// Handle the parsedResult to perform Kubernetes operations
	},
}

func init() {
	rootCmd.AddCommand(queryCmd)

	// No need to set up a flag here, as we're using arguments
}
