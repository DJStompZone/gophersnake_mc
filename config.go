package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	
	"github.com/google/uuid"
)

// Config holds all configuration settings for the application
type Config struct {
	MinecraftServer struct {
		Address     string `json:"address"`
		Port        int    `json:"port"`
		OnlineMode  bool   `json:"online_mode"`
		GameVersion string `json:"game_version"`
	} `json:"minecraft_server"`
	
	WebSocket struct {
		Address string `json:"address"`
		Port    int    `json:"port"`
	} `json:"websocket"`
	
	Player struct {
		DisplayName string `json:"display_name"`
		DeviceID    string `json:"device_id"`
	} `json:"player"`
}

var config Config

// LoadConfig loads configuration from the config.json file
func LoadConfig() error {
	// Default configuration
	config = Config{}
	config.MinecraftServer.Address = "localhost"
	config.MinecraftServer.Port = 19132
	config.MinecraftServer.OnlineMode = false
	config.MinecraftServer.GameVersion = "1.20.0"
	
	config.WebSocket.Address = "0.0.0.0"
	config.WebSocket.Port = 8080
	
	config.Player.DisplayName = "GopherSnake"
	config.Player.DeviceID = uuid.New().String()
	
	// Check if config file exists
	if _, err := os.Stat("config.json"); os.IsNotExist(err) {
		// Create default config file
		configData, err := json.MarshalIndent(config, "", "    ")
		if err != nil {
			return err
		}
		
		err = ioutil.WriteFile("config.json", configData, 0644)
		if err != nil {
			return err
		}
		
		log.Println("Created default configuration file: config.json")
		return nil
	}
	
	// Read existing config file
	configData, err := ioutil.ReadFile("config.json")
	if err != nil {
		return err
	}
	
	err = json.Unmarshal(configData, &config)
	if err != nil {
		return err
	}
	
	log.Println("Loaded configuration from config.json")
	return nil
}

func GetMinecraftServerAddress() string {
	return fmt.Sprintf("%s:%d", config.MinecraftServer.Address, config.MinecraftServer.Port)
}

func GetWebSocketAddress() string {
	return fmt.Sprintf("%s:%d", config.WebSocket.Address, config.WebSocket.Port)
}
