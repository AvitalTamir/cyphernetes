package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// shellCmd represents the shell command
var shellCmd = &cobra.Command{
	Use:   "shell",
	Short: "Launch the Cyphernetes interactive shell",
	Long:  `Use the 'shell' subcommand to enter an interactive shell for executing multiple Cypher-inspired queries against your Kubernetes resources.`,
	Run: func(cmd *cobra.Command, args []string) {
		// This is where you would initialize and run your interactive shell
		fmt.Println("Entering interactive shell...")
	},
}

func init() {
	rootCmd.AddCommand(shellCmd)

	// Here you can define flags and configuration settings for the 'shell' subcommand if needed
}
