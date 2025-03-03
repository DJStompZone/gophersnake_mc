package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"

	"github.com/sandertv/gophertunnel/minecraft/auth"
)

// AuthenticateWithDeviceCode performs device code authentication and returns a Minecraft JWT chain
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
