package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"fmt"
	"log"
	"os/exec"

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

	// If on Windows, try multiple Python command variants
	if GetRuntimeOS() == "windows" {
		return "python.exe"
		// Try 'py' first (Python launcher), then 'python'
		// cmds := []string{"python", "python3", "py"}
		// for _, cmd := range cmds {
		// 	if _, err := exec.LookPath(cmd); err == nil {
		// 		log.Printf("Found Python command: %s", cmd)
		// 		return cmd
		// 	}
		// }
		// Default to python if none found
		// log.Printf("Warning: Could not find Python command, defaulting to 'python'")
		// return "python"
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
	log.Println("Starting device code authentication via gophertunnel...")

	// Use the built-in TokenSource to get a Live token
	log.Println("Requesting Microsoft Live token...")
	ctx := context.Background()

	// Request Live token using device code flow
	liveToken, err := auth.RequestLiveToken()
	if err != nil {
		return "", nil, fmt.Errorf("failed to get Live token: %w", err)
	}
	log.Println("Successfully obtained Microsoft Live token")

	// Generate ECDSA private key for encryption
	log.Println("Generating ECDSA key for Minecraft authentication...")
	privateKey, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		return "", nil, fmt.Errorf("failed to generate ECDSA key: %w", err)
	}

	// Get XBL token with Minecraft relying party
	log.Println("Requesting XBL token...")
	xblToken, err := auth.RequestXBLToken(ctx, liveToken, "https://multiplayer.minecraft.net/")
	if err != nil {
		log.Printf("Failed with Minecraft relying party: %v", err)
		log.Println("Falling back to general Xbox Live relying party...")

		// Fall back to general Xbox Live relying party
		xblToken, err = auth.RequestXBLToken(ctx, liveToken, "http://xboxlive.com")
		if err != nil {
			return "", nil, fmt.Errorf("failed to get XBL token: %w", err)
		}
	}

	// Log the user hash for debugging
	if len(xblToken.AuthorizationToken.DisplayClaims.UserInfo) > 0 {
		userHash := xblToken.AuthorizationToken.DisplayClaims.UserInfo[0].UserHash
		log.Printf("Successfully obtained XBL token with UHS: %s", userHash)
	} else {
		log.Println("Warning: XBL token obtained but missing user hash")
	}

	// Request Minecraft chain
	log.Println("Requesting Minecraft authentication chain...")
	minecraftChain, err := auth.RequestMinecraftChain(ctx, xblToken, privateKey)
	if err != nil {
		return "", nil, fmt.Errorf("failed to get Minecraft chain: %w", err)
	}
	log.Printf("Successfully obtained Minecraft chain (length: %d characters)", len(minecraftChain))

	return minecraftChain, privateKey, nil
}

// AuthenticateWithPythonXBL3 obtains an XBL3.0 token and creates an ECDSA key
// using the built-in gophertunnel auth functionality
func AuthenticateWithPythonXBL3() (*auth.XBLToken, *ecdsa.PrivateKey, error) {
	log.Println("Starting authentication using gophertunnel's built-in auth...")

	// Generate ECDSA key for Minecraft authentication
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate ECDSA key: %w", err)
	}

	// Use the built-in TokenSource from gophertunnel
	ctx := context.Background()

	// Request Live token using device code flow
	log.Println("Requesting Microsoft Live token...")
	liveToken, err := auth.RequestLiveToken()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get Live token: %w", err)
	}
	log.Println("Successfully obtained Microsoft Live token")

	// Try different relying parties
	relyingParties := []string{
		"https://multiplayer.minecraft.net/",
		"http://xboxlive.com",        // General Xbox Live
		"https://sisu.xboxlive.com/", // The SISU endpoint itself
	}

	var xblToken *auth.XBLToken
	var lastErr error

	for _, rp := range relyingParties {
		log.Printf("Trying relying party: %s", rp)
		xblToken, err = auth.RequestXBLToken(ctx, liveToken, rp)
		if err != nil {
			log.Printf("Failed with relying party %s: %v", rp, err)
			lastErr = err
			continue
		}

		// Success
		log.Printf("Successfully obtained XBL token with relying party: %s", rp)

		// Verify we got a valid token with user hash
		if len(xblToken.AuthorizationToken.DisplayClaims.UserInfo) == 0 {
			log.Printf("Warning: XBL token with relying party %s is missing user information", rp)
			lastErr = fmt.Errorf("XBL token missing user information")
			continue
		}

		// Valid token found
		userHash := xblToken.AuthorizationToken.DisplayClaims.UserInfo[0].UserHash
		log.Printf("Successfully obtained XBL token with UHS: %s", userHash)
		return xblToken, privateKey, nil
	}

	// All relying parties failed
	return nil, nil, fmt.Errorf("failed to get XBL token with any relying party: %w", lastErr)
}

// startDeviceCodeFlow is not needed anymore - we're using the gophertunnel implementation directly
