package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"fmt"
	"log"
	"os/exec"
	"strings"

	"github.com/sandertv/gophertunnel/minecraft/auth"
)

// CheckPythonDependencies verifies that the required Python packages are installed
func CheckPythonDependencies() error {
	// Use the same Python command detection logic as in GetXBL3Token
	pythonCmd := GetPythonCommand()

	// install requests and msal
	cmd := exec.Command(pythonCmd, "-m", "pip", "install", "requests", "msal")
	if err := cmd.Run(); err != nil {
		log.Printf("Failed to install required Python packages: %v", err)
		return fmt.Errorf("failed to install required Python packages: %w", err)
	}

	// Check for required packages
	requiredPackages := []string{"requests", "msal"}
	for _, pkg := range requiredPackages {
		cmd := exec.Command(pythonCmd, "-c", fmt.Sprintf("import %s", pkg))
		if err := cmd.Run(); err != nil {
			log.Printf("Python package '%s' not found. Please install it using: pip install %s", pkg, pkg)
			return fmt.Errorf("missing required Python package: %s", pkg)
		}
	}
	return nil
}

// GetPythonCommand returns the appropriate Python command for the current OS
func GetPythonCommand() string {
	return "python.exe"
	// If on Windows, try multiple Python command variants
	if GetRuntimeOS() == "windows" {
		// Try 'py' first (Python launcher), then 'python'
		cmds := []string{"python", "python3", "py"}
		for _, cmd := range cmds {
			if _, err := exec.LookPath(cmd); err == nil {
				log.Printf("Found Python command: %s", cmd)
				return cmd
			}
		}
		// Default to python if none found
		log.Printf("Warning: Could not find Python command, defaulting to 'python'")
		return "python"
	}

	// On non-Windows platforms, use python3
	return "python3"
}

// GetRuntimeOS returns the current operating system (abstracted for testing)
func GetRuntimeOS() string {
	return "windows" // Hard-coded for now, but should get from runtime.GOOS
}

// AuthenticateWithDeviceCode performs device code authentication and returns a Minecraft JWT chain
// using the built-in gophertunnel auth flow
func AuthenticateWithDeviceCode(clientID string) (string, *ecdsa.PrivateKey, error) {
	// Request Live token using device code flow
	liveToken, err := auth.RequestLiveToken()
	if err != nil {
		return "", nil, err
	}

	// Request XBOX Live token
	xblToken, err := auth.RequestXBLToken(context.Background(), liveToken, "rp://api.minecraftservices.com/")
	if err != nil {
		return "", nil, err
	}

	// Generate ECDSA private key for encryption
	privateKey, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		return "", nil, err
	}

	// Request Minecraft chain
	minecraftChain, err := auth.RequestMinecraftChain(context.Background(), xblToken, privateKey)
	if err != nil {
		return "", nil, err
	}

	return minecraftChain, privateKey, nil
}

// AuthenticateWithPythonXBL3 performs authentication using the Python XBL3.0 token
// script and returns a Minecraft JWT chain
func AuthenticateWithPythonXBL3() (string, *ecdsa.PrivateKey, error) {
	log.Println("Starting XBL3.0 authentication process using Python script...")

	// Check Python dependencies first
	if err := CheckPythonDependencies(); err != nil {
		log.Printf("Python dependency check failed: %v", err)
		log.Println("Please install required packages with: pip install msal requests")
		return "", nil, fmt.Errorf("python dependency check failed: %w", err)
	}

	// Get XBL3.0 token from Python script
	xbl3TokenStr, err := GetXBL3Token()
	if err != nil {
		log.Printf("Error getting XBL3.0 token: %v", err)

		// Check if Python is installed
		pythonCmd := GetPythonCommand()
		checkCmd := exec.Command(pythonCmd, "--version")
		if versionErr := checkCmd.Run(); versionErr != nil {
			log.Printf("Python not found or not working correctly: %v", versionErr)
			return "", nil, fmt.Errorf("python not found or not working: %w", versionErr)
		}

		return "", nil, fmt.Errorf("failed to get XBL3.0 token: %w", err)
	}

	if xbl3TokenStr == "" {
		return "", nil, fmt.Errorf("received empty XBL3.0 token from Python script")
	}

	// Validate token format
	if !strings.HasPrefix(xbl3TokenStr, "XBL3.0") {
		log.Printf("Warning: Unexpected token format: '%s'", xbl3TokenStr)
		return "", nil, fmt.Errorf("invalid token format: does not start with XBL3.0")
	}

	// Parse the token into the expected format
	xblToken, err := parseXBL3TokenToStruct(xbl3TokenStr)
	if err != nil {
		return "", nil, fmt.Errorf("failed to parse XBL3.0 token: %w", err)
	}

	// Generate ECDSA private key for encryption
	privateKey, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		return "", nil, fmt.Errorf("failed to generate EC private key: %w", err)
	}

	log.Println("Requesting Minecraft chain using XBL3.0 token...")

	// Request Minecraft chain using the XBL3.0 token
	minecraftChain, err := auth.RequestMinecraftChain(context.Background(), xblToken, privateKey)
	if err != nil {
		return "", nil, fmt.Errorf("failed to obtain Minecraft chain: %w", err)
	}

	log.Println("Successfully obtained Minecraft authentication chain")
	return minecraftChain, privateKey, nil
}

// parseXBL3TokenToStruct parses a XBL3.0 token string into the auth.XBLToken struct
func parseXBL3TokenToStruct(tokenStr string) (*auth.XBLToken, error) {
	// Clean and validate input
	tokenStr = strings.TrimSpace(tokenStr)

	if tokenStr == "" {
		return nil, fmt.Errorf("empty token string")
	}

	if !strings.HasPrefix(tokenStr, "XBL3.0") {
		return nil, fmt.Errorf("invalid token format: missing XBL3.0 prefix")
	}

	// Remove the "XBL3.0 " prefix
	tokenStr = strings.TrimPrefix(tokenStr, "XBL3.0 ")

	// Split the "x=<uhs>;<token>" part
	parts := strings.Split(tokenStr, ";")
	if len(parts) != 2 {
		return nil, fmt.Errorf("unexpected XBL3.0 token format: should be 'XBL3.0 x=<uhs>;<token>'")
	}

	// Extract the UHS (user hash) part
	uhsPart := parts[0]
	if !strings.HasPrefix(uhsPart, "x=") {
		return nil, fmt.Errorf("unexpected UHS format in token: missing 'x=' prefix")
	}

	uhsPart = strings.TrimPrefix(uhsPart, "x=")
	if uhsPart == "" {
		return nil, fmt.Errorf("empty UHS value in token")
	}

	// Extract the token part
	xstsToken := parts[1]
	if xstsToken == "" {
		return nil, fmt.Errorf("empty XSTS token value")
	}

	// Create a properly formatted XBLToken struct
	xblToken := &auth.XBLToken{}

	// Set the token in the expected format
	xblToken.AuthorizationToken.Token = xstsToken

	// Set the user hash in the expected format
	userInfo := struct {
		GamerTag string `json:"gtg"`
		XUID     string `json:"xid"`
		UserHash string `json:"uhs"`
	}{
		UserHash: uhsPart,
	}

	xblToken.AuthorizationToken.DisplayClaims.UserInfo = []struct {
		GamerTag string `json:"gtg"`
		XUID     string `json:"xid"`
		UserHash string `json:"uhs"`
	}{userInfo}

	log.Printf("Successfully parsed XBL3.0 token with UHS: %s", uhsPart)
	return xblToken, nil
}
