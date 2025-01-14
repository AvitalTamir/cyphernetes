/*
Copyright © 2023 Avital Tamir <avital.osog@gmail.com>
*/
package main

import (
	"fmt"
	"os"

	"github.com/avitaltamir/cyphernetes/pkg/core"
	"github.com/spf13/cobra"
)

var (
	Version = "dev"
	DryRun  = false
)

func getVersionInfo() string {
	return fmt.Sprintf(
		"Cyphernetes %s\n"+
			"License: Apache 2.0\n"+
			"Source: https://github.com/avitaltamir/cyphernetes\n",
		Version,
	)
}

var LogLevel = "info"

// rootCmd represents the base command when called without any subcommands
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
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Only run version check here
		versionFlag, _ := cmd.Flags().GetBool("version")
		if versionFlag {
			fmt.Print(getVersionInfo())
			os.Exit(0)
		}
	},
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
	// First set the log level
	rootCmd.PersistentFlags().StringVarP(&LogLevel, "loglevel", "l", "info", "The log level to use (debug, info, warn, error, fatal, panic)")

	// Move format validation to a separate function
	rootCmd.PersistentPreRunE = validateGlobalFlags

	// Add other flags
	rootCmd.PersistentFlags().StringVarP(&core.Namespace, "namespace", "n", "default", "The namespace to query against")
	rootCmd.PersistentFlags().BoolVarP(&core.AllNamespaces, "all-namespaces", "A", false, "Query all namespaces")
	rootCmd.PersistentFlags().BoolVar(&core.NoColor, "no-color", false, "Disable colored output in shell and query results")
	rootCmd.PersistentFlags().BoolP("version", "v", false, "Show version and exit")
	rootCmd.PersistentFlags().BoolVar(&DryRun, "dry-run", false, "Enable dry-run mode for all operations")

	// Add version command
	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Print(getVersionInfo())
		},
	})

	// Add completion command
	rootCmd.AddCommand(&cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate completion script",
		Long: `To load completions:

Bash:
  $ source <(cyphernetes completion bash)

Zsh:
  $ source <(cyphernetes completion zsh)

fish:
  $ cyphernetes completion fish | source

PowerShell:
  PS> cyphernetes completion powershell | Out-String | Invoke-Expression
`,
		ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
		Args:      cobra.ExactValidArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			switch args[0] {
			case "bash":
				cmd.Root().GenBashCompletion(os.Stdout)
			case "zsh":
				cmd.Root().GenZshCompletion(os.Stdout)
			case "fish":
				cmd.Root().GenFishCompletion(os.Stdout, true)
			case "powershell":
				cmd.Root().GenPowerShellCompletionWithDesc(os.Stdout)
			}
		},
	})

	// Add the web command
	rootCmd.AddCommand(WebCmd)
}

// validateGlobalFlags validates global flags without initializing k8s
func validateGlobalFlags(cmd *cobra.Command, args []string) error {
	// Set LogLevel after flag parsing
	LogLevel = cmd.Flag("loglevel").Value.String()
	core.LogLevel = LogLevel

	return nil
}

// initializeKubernetes initializes kubernetes context - call this in commands that need k8s
func initializeKubernetes() error {
	// Add any kubernetes initialization code here
	return nil
}

func logDebug(v ...interface{}) {
	if core.LogLevel == "debug" {
		fmt.Println(append([]interface{}{"[DEBUG] "}, v...)...)
	}
}
