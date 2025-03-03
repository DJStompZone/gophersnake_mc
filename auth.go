package main

import (
	"context"

	"github.com/sandertv/gophertunnel/minecraft/auth"
)

// AuthenticateWithDeviceCode performs device code authentication and returns a Minecraft JWT chain
func AuthenticateWithDeviceCode(clientID string) (string, error) {
	// Request Live token using device code flow
	liveToken, err := auth.RequestLiveToken()
	if err != nil {
		return "", err
	}

	// Request XBOX Live token
	xblToken, err := auth.RequestXBLToken(context.Background(), liveToken, "rp://api.minecraftservices.com/")
	if err != nil {
		return "", err
	}

	// Request Minecraft chain
	minecraftChain, err := auth.RequestMinecraftChain(context.Background(), xblToken, nil)
	if err != nil {
		return "", err
	}

	return minecraftChain, nil
}
