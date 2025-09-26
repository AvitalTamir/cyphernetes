package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/avitaltamir/cyphernetes/pkg/notebook"
	"github.com/spf13/cobra"
)

var (
	notebookPort    int
	notebookDataDir string
)

var notebookCmd = &cobra.Command{
	Use:   "notebook",
	Short: "Start the Cyphernetes notebook server",
	Long: `Start a Jupyter-style notebook server for Cyphernetes.

The notebook server provides:
- Interactive notebook interface for running Cyphernetes queries
- Multiple visualization formats (JSON, YAML, Table, Graph)
- Persistent storage of notebooks and results`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Create notebook server config
		config := notebook.ServerConfig{
			Port:    notebookPort,
			DataDir: notebookDataDir,
		}

		// Create and start the server
		server, err := notebook.NewServer(config)
		if err != nil {
			return fmt.Errorf("failed to create notebook server: %w", err)
		}

		// Handle graceful shutdown
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

		// Start server in background
		errChan := make(chan error, 1)
		go func() {
			errChan <- server.Start()
		}()

		fmt.Printf("🚀 Cyphernetes Notebook server starting on http://localhost:%d\n", notebookPort)
		fmt.Printf("📁 Data directory: %s\n", notebookDataDir)

		// Wait for shutdown signal or error
		select {
		case err := <-errChan:
			return fmt.Errorf("server error: %w", err)
		case <-sigChan:
			fmt.Println("\n🛑 Shutting down notebook server...")
			return server.Stop()
		}
	},
}

var notebookShareCmd = &cobra.Command{
	Use:   "share",
	Short: "Generate a pin code to share your notebook",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := notebook.NewClient(fmt.Sprintf("http://localhost:%d", notebookPort))
		pin, err := client.GenerateSharePin()
		if err != nil {
			return fmt.Errorf("failed to generate share pin: %w", err)
		}

		fmt.Printf("📌 Share Pin: %s\n", pin)
		fmt.Println("\nThis pin is valid for 10 minutes.")
		fmt.Println("Others can connect using: cyphernetes notebook connect " + pin)
		return nil
	},
}

var notebookConnectCmd = &cobra.Command{
	Use:   "connect [pin]",
	Short: "Connect to a shared notebook using a pin",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		pin := args[0]

		// TODO: Parse pin to extract connection details
		// TODO: Open browser to remote notebook

		fmt.Printf("🔗 Connecting to notebook with pin: %s\n", pin)
		return fmt.Errorf("connect command not yet implemented")
	},
}

func init() {
	// Add flags for notebook command
	notebookCmd.Flags().IntVarP(&notebookPort, "port", "p", 8080, "Port to run the notebook server on")
	notebookCmd.Flags().StringVarP(&notebookDataDir, "data-dir", "d", "~/.cyphernetes/notebooks", "Directory to store notebook data")

	// Add subcommands
	notebookCmd.AddCommand(notebookShareCmd)
	notebookCmd.AddCommand(notebookConnectCmd)

	// Add to root command
	rootCmd.AddCommand(notebookCmd)
}
