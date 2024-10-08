package main

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"

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

	router := gin.Default()

	// Serve embedded files from the 'web' directory
	webContent, err := fs.Sub(webFS, "web")
	if err != nil {
		fmt.Printf("Error accessing embedded web files: %v\n", err)
		return
	}
	router.StaticFS("/", http.FS(webContent))

	// Setup API routes
	setupAPIRoutes(router)

	// Start the server
	fmt.Printf("Starting Cyphernetes web interface at %s\n", url)
	if err := router.Run(":" + port); err != nil {
		fmt.Printf("Error starting server: %v\n", err)
		return
	}
}
