package main

import (
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// Helper function to fix missing TargetName in Text packet
func setTextPacketTarget(textPacket *packet.Text, target string) {
	textPacket.Parameters = append(textPacket.Parameters, target)
}

// GetGameVersion returns the game version from config or a default value
func GetGameVersion() string {
	if config.MinecraftServer.GameVersion != "" {
		return config.MinecraftServer.GameVersion
	}
	return "1.21.62" // Default version
}
