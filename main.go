package main

import (
	"log"
)

// This function is a simpler version of setup() without the HTTP server
func setup() {
	log.Println("Starting GopherSnake MC setup...")
	
	// Configure logging
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	
	// Load configuration
	if err := LoadConfig(); err != nil {
		log.Fatalf("Error loading configuration: %v", err)
	}
}
