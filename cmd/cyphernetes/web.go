package main

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/avitaltamir/cyphernetes/pkg/core"
	"github.com/avitaltamir/cyphernetes/pkg/provider/apiserver"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
)

//go:embed web/*
var webFS embed.FS

var WebCmd = &cobra.Command{
	Use:   "web",
	Short: "Start the Cyphernetes web interface",
	Long: `Start the Cyphernetes web interface.

If the specified port is in use, it will attempt to find the next available port.`,
	Run: runWeb,
}

var (
	webPort    string
	maxPortTry = 10 // Maximum number of ports to try
)

func init() {
	WebCmd.Flags().StringVarP(&webPort, "port", "p", "8080", "Port to run the web interface on")
}

// checkPort attempts to listen on a port to check if it's available
func checkPort(port string) error {
	// Try to bind to all interfaces, just like the actual server will
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return err
	}
	listener.Close()
	return nil
}

// findAvailablePort finds the next available port starting from the given port
func findAvailablePort(startPort string) (string, error) {
	port, err := strconv.Atoi(startPort)
	if err != nil {
		return "", fmt.Errorf("invalid port number: %s", startPort)
	}

	for i := 0; i < maxPortTry; i++ {
		currentPort := strconv.Itoa(port + i)
		err := checkPort(currentPort)
		if err == nil {
			return currentPort, nil
		}
	}
	return "", fmt.Errorf("no available ports found in range %d-%d", port, port+maxPortTry-1)
}

func runWeb(cmd *cobra.Command, args []string) {
	// Find an available port
	port, err := findAvailablePort(webPort)
	if err != nil {
		fmt.Printf("Error finding available port: %v\n", err)
		os.Exit(1)
	}

	// If we're using a different port than requested, inform the user
	if port != webPort {
		fmt.Printf("Port %s is in use, using port %s instead\n", webPort, port)
	}

	url := fmt.Sprintf("http://localhost:%s", port)

	// Create the API server provider
	providerConfig := &apiserver.APIServerProviderConfig{
		DryRun: DryRun,
	}
	provider, err := apiserver.NewAPIServerProviderWithOptions(providerConfig)
	if err != nil {
		fmt.Printf("Error creating provider: %v\n", err)
		os.Exit(1)
	}

	// Initialize the executor instance with the provider
	executor = core.GetQueryExecutorInstance(provider)
	if executor == nil {
		fmt.Printf("Error initializing query executor\n")
		return
	}

	// Set Gin to release mode to disable logging
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()

	// Setup API routes first
	setupAPIRoutes(router)

	// Serve embedded files from the 'web' directory
	webContent, err := fs.Sub(webFS, "web")
	if err != nil {
		fmt.Printf("Error accessing embedded web files: %v\n", err)
		return
	}
	router.NoRoute(gin.WrapH(http.FileServer(http.FS(webContent))))

	// Create a new http.Server
	srv := &http.Server{
		Addr:    ":" + port, // Bind to all interfaces for better compatibility
		Handler: router,
	}

	// Create a channel to signal when the server has finished shutting down
	serverClosed := make(chan struct{})

	// Start the server in a goroutine
	go func() {
		fmt.Printf("\nStarting Cyphernetes web interface at %s\n", url)
		fmt.Println("Press Ctrl+C to stop")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Error starting server: %v\n", err)
		}
		close(serverClosed)
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("Shutting down server...")

	// Create a deadline to wait for.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Doesn't block if no connections, but will otherwise wait
	// until the timeout deadline.
	if err := srv.Shutdown(ctx); err != nil {
		fmt.Printf("Server forced to shutdown: %v\n", err)
	}

	// Wait for the server to finish shutting down
	<-serverClosed

	fmt.Println("Server exiting")
}
