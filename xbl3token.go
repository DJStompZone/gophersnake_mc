package main

import (
	"bytes"
	"log"
	"os/exec"
	"strings"
)

// GetXBL3Token runs the xbl3cli.py script and returns the XBL3.0 token.
// It uses the GetPythonCommand function to determine the appropriate Python command.
func GetXBL3Token() (string, error) {
	// Get the appropriate Python command for the current platform
	pythonCmd := GetPythonCommand()
	log.Printf("Using Python command: %s", pythonCmd)

	// Create command and configure separate streams for stdout and stderr
	cmd := exec.Command(pythonCmd, "xbl3cli.py")

	// Capture stdout (token) and stderr (debug info) separately
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run the command
	err := cmd.Run()

	// Log any stderr output for debugging
	if stderrStr := stderr.String(); stderrStr != "" {
		log.Printf("Python script debug output: %s", stderrStr)
	}

	// Check for errors
	if err != nil {
		log.Printf("Error running Python script: %v", err)
		return "", err
	}

	// Get token from stdout
	token := strings.TrimSpace(stdout.String())

	// Validate token format
	if token == "" {
		log.Printf("Python script returned empty token")
		return "", err
	}

	if !strings.HasPrefix(token, "XBL3.0") {
		log.Printf("Warning: Token doesn't start with expected 'XBL3.0' prefix: %s", token)
	}

	log.Printf("Successfully obtained XBL3.0 token")

	// Log a truncated version of the token for debugging
	if len(token) > 20 {
		log.Printf("Token starts with: %s...", token[:20])
	}

	return token, nil
}
