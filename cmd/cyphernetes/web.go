package main

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/avitaltamir/cyphernetes/pkg/core"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
)

//go:embed web/*
var webFS embed.FS

var WebCmd = &cobra.Command{
	Use:   "web",
	Short: "Start the Cyphernetes web interface",
	Run:   runWeb,
}

func runWeb(cmd *cobra.Command, args []string) {
	port := "8080"
	url := fmt.Sprintf("http://localhost:%s", port)

	core.InitResourceSpecs()
	resourceSpecs = core.ResourceSpecs

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
		Addr:    ":" + port,
		Handler: router,
	}

	// Create a channel to signal when the server has finished shutting down
	serverClosed := make(chan struct{})

	// Start the server in a goroutine
	go func() {
		fmt.Printf("Starting Cyphernetes web interface at %s\n", url)
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
