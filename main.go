package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	log.Println("Starting GopherSnake MC...")
	
	// Configure logging
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	
	// Load configuration
	if err := LoadConfig(); err != nil {
		log.Fatalf("Error loading configuration: %v", err)
	}
	
	// Start the Minecraft connection manager
	go connectMinecraft()

	// Setup WebSocket route
	http.HandleFunc("/chat", handleConnections)
	
	// Setup health check endpoint
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Start HTTP server
	wsAddr := GetWebSocketAddress()
	log.Printf("Starting WebSocket server on %s", wsAddr)
	
	// Handle graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		
		log.Println("Shutting down...")
		os.Exit(0)
	}()
	
	// Start the server
	err := http.ListenAndServe(wsAddr, nil)
	if err != nil {
		log.Fatal("Error starting HTTP server:", err)
	}
}
