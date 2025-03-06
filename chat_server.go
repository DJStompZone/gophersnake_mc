// chat_server.go
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/auth"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/login"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"golang.org/x/oauth2"
)

var (
	clients    = make(map[*websocket.Conn]bool) // Connected WebSocket clients
	clientsMux sync.Mutex                       // Mutex for the clients map
	upgrader   = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true // Allow all connections
		},
	}
	minecraftConn *minecraft.Conn // Connection to Minecraft server
)

// Message represents a chat message structure
type Message struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Sender  string `json:"sender,omitempty"`
	Target  string `json:"target,omitempty"`
}

func main() {
	// Call setup function from main.go if it exists
	// Otherwise, comment out this line
	setup()

	// Start the Minecraft connection manager
	go connectMinecraft()

	// Setup WebSocket route
	http.HandleFunc("/chat", handleConnections)

	// Start HTTP server
	addr := GetWebSocketAddress()
	log.Printf("Starting WebSocket server at %s...", addr)
	err := http.ListenAndServe(addr, nil)
	if err != nil {
		log.Fatal("Error starting server:", err)
	}
}

// handleConnections handles incoming WebSocket connections
func handleConnections(w http.ResponseWriter, r *http.Request) {
	// Upgrade initial GET request to a WebSocket
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Error upgrading to WebSocket: %v", err)
		return
	}
	defer ws.Close()

	// Register new client
	clientsMux.Lock()
	clients[ws] = true
	clientCount := len(clients)
	clientsMux.Unlock()
	log.Printf("New WebSocket client connected. Total clients: %d", clientCount)

	// Welcome message
	welcomeMsg := Message{Type: "info", Message: fmt.Sprintf("Welcome! Connected to GopherSnake MC. There are %d client(s) connected.", clientCount)}
	err = ws.WriteJSON(welcomeMsg)
	if err != nil {
		log.Printf("Error sending welcome message: %v", err)
		return
	}

	// Main message handling loop
	for {
		var msg Message
		err := ws.ReadJSON(&msg)
		if err != nil {
			log.Printf("Error reading message: %v", err)
			clientsMux.Lock()
			delete(clients, ws)
			clientsMux.Unlock()
			break
		}

		// Process the message based on its type
		switch msg.Type {
		case "chat":
			// Forward message to Minecraft
			log.Printf("Received chat message from WebSocket: %s", msg.Message)
			sendChatToMinecraft(msg.Message, msg.Target)
			broadcastToWebsocket(msg)
		default:
			log.Printf("Unknown message type: %s", msg.Type)
		}
	}
}

// broadcastToWebsocket sends a message to all connected WebSocket clients
func broadcastToWebsocket(msg Message) {
	clientsMux.Lock()
	defer clientsMux.Unlock()

	for client := range clients {
		err := client.WriteJSON(msg)
		if err != nil {
			log.Printf("Error broadcasting message: %v", err)
			client.Close()
			delete(clients, client)
		}
	}
}

// connectMinecraft establishes a connection to the Minecraft Bedrock server
func connectMinecraft() {
	for {
		log.Println("Attempting to connect to Minecraft server...")

		serverAddr := GetMinecraftServerAddress()
		if serverAddr == "" {
			log.Println("Invalid Minecraft server address in config.json")
			return
		}

		// Create a dialer for connecting to the Minecraft server
		dialer := minecraft.Dialer{
			ClientData: login.ClientData{
				GameVersion:      GetGameVersion(),
				DeviceOS:         protocol.DeviceAndroid,
				DeviceID:         config.Player.DeviceID,
				DeviceModel:      "GopherSnake MC",
				DefaultInputMode: 2, // Touch input mode (2)
				CurrentInputMode: 2, // Touch input mode (2)
				UIProfile:        0, // Classic UI (0)
				GUIScale:         0, // Default GUI scale
				LanguageCode:     "en_US",
			},
			IdentityData: login.IdentityData{
				DisplayName: config.Player.DisplayName,
			},
		}

		// Check if online mode is enabled in config
		if config.MinecraftServer.OnlineMode {
			log.Println("Online mode enabled, performing authentication...")

			// IMPORTANT: Use the built-in TokenSource directly as intended
			log.Println("Using gophertunnel's built-in auth.TokenSource for authentication")
			dialer.TokenSource = auth.TokenSource
		}

		log.Println("Dialing Minecraft server...")

		// Attempt to dial the Minecraft server
		conn, err := dialer.Dial("raknet", serverAddr)
		if err != nil {
			logMinecraftErrorDiagnostics(err)

			// If the default auth.TokenSource failed, try with our custom implementation
			if config.MinecraftServer.OnlineMode {
				log.Println("Default auth failed, trying custom authentication...")

				xblToken, key, err := AuthenticateWithPythonXBL3()
				if err != nil {
					log.Printf("Custom authentication failed: %v", err)
					time.Sleep(10 * time.Second)
					continue
				}

				// Request the Minecraft chain
				chain, err := auth.RequestMinecraftChain(context.Background(), xblToken, key)
				if err != nil {
					log.Printf("Failed to obtain Minecraft chain: %v", err)
					time.Sleep(10 * time.Second)
					continue
				}

				// Use our wrapper for the token source
				log.Println("Using custom token source for authentication")
				dialer.TokenSource = &tokenSourceWrapper{chain: chain}

				// Try dialing again with our custom token source
				log.Println("Retrying connection with custom authentication...")
				conn, err = dialer.Dial("raknet", serverAddr)
				if err != nil {
					logMinecraftErrorDiagnostics(err)
					time.Sleep(10 * time.Second)
					continue
				}
			} else {
				// Not in online mode, just wait and retry
				time.Sleep(10 * time.Second)
				continue
			}
		}

		log.Printf("Successfully connected to Minecraft server at %s", serverAddr)
		minecraftConn = conn

		// Handle incoming Minecraft packets
		go handleMinecraftConnection(conn)
	}
}

// tokenSourceWrapper wraps a Minecraft chain string to implement oauth2.TokenSource
type tokenSourceWrapper struct {
	chain string
}

// Token implements the oauth2.TokenSource interface
func (t *tokenSourceWrapper) Token() (*oauth2.Token, error) {
	// Create a token with the chain in the AccessToken field
	// This ensures the chain is properly passed to gophertunnel
	return &oauth2.Token{
		AccessToken: t.chain,
		TokenType:   "Bearer",
		// Set a far future expiry time since Minecraft chains don't expire quickly
		Expiry: time.Now().Add(24 * time.Hour),
	}, nil
}

// logMinecraftErrorDiagnostics provides detailed diagnostic information for Minecraft connection errors
func logMinecraftErrorDiagnostics(err error) {
	// Log the error in more detail.
	log.Printf("Diagnostic details: %v", err)

	// Log game version from configuration.
	version := GetGameVersion()
	log.Printf("Configured game version: %s", version)

	// If the error message contains known markers, print additional hints.
	if strings.Contains(err.Error(), "client outdated") {
		log.Println("Hint: The server reports the client as outdated. Check if the game version matches.")
	}

	// If possible, output a formatted error (if err implements fmt.Formatter)
	log.Printf("Full error: %+v", err)
}

// handleMinecraftConnection processes incoming packets from the Minecraft server
func handleMinecraftConnection(conn *minecraft.Conn) {
	defer func() {
		log.Println("Minecraft connection closed, attempting to reconnect...")
		// Try to reconnect
		connectMinecraft()
	}()

	for {
		pk, err := conn.ReadPacket()
		if err != nil {
			log.Printf("Error reading packet: %v", err)
			return
		}

		// Handle different packet types
		switch p := pk.(type) {
		case *packet.Text:
			handleMinecraftChatMessage(p)
		// Add other packet types as needed
		default:
			// Uncomment for verbose packet logging
			// log.Printf("Received packet of type %T: %+v", pk, pk)
		}
	}
}

// handleMinecraftChatMessage processes incoming chat messages from Minecraft
func handleMinecraftChatMessage(textPacket *packet.Text) {
	// Filter out system messages if needed
	if textPacket.TextType != packet.TextTypeChat {
		return
	}

	// Create message object for WebSocket
	msg := Message{
		Type:    "chat",
		Message: textPacket.Message,
		Sender:  textPacket.SourceName,
	}

	// Broadcast to all WebSocket clients
	broadcastToWebsocket(msg)
	log.Printf("[MC] %s: %s", textPacket.SourceName, textPacket.Message)
}

// sendChatToMinecraft sends a chat message to the Minecraft server
func sendChatToMinecraft(message string, target string) {
	if minecraftConn == nil {
		log.Println("Cannot send message: No Minecraft connection")
		return
	}

	// Create chat packet
	textPacket := &packet.Text{
		TextType:         packet.TextTypeChat,
		NeedsTranslation: false,
		SourceName:       config.Player.DisplayName,
		Message:          message,
	}

	// Set target if specified
	if target != "" {
		textPacket.XUID = target
	}

	// Send the packet
	err := minecraftConn.WritePacket(textPacket)
	if err != nil {
		log.Printf("Error sending chat message to Minecraft: %v", err)
	}
}

// getClientData returns a ClientData struct configured for connecting to Minecraft
func getClientData() login.ClientData {
	return login.ClientData{
		GameVersion:      GetGameVersion(),
		DeviceOS:         protocol.DeviceAndroid,
		DeviceID:         config.Player.DeviceID,
		DefaultInputMode: 2, // Touch input mode (2)
		CurrentInputMode: 2, // Touch input mode (2)
		UIProfile:        0, // Classic UI (0)
		GUIScale:         0, // Default GUI scale
		LanguageCode:     "en_US",
	}
}

// getIdentityData returns a basic IdentityData struct, which will be populated during authentication
func getIdentityData() login.IdentityData {
	return login.IdentityData{
		DisplayName: config.Player.DisplayName,
		Identity:    "", // Will be filled during authentication
		XUID:        "", // Will be filled during authentication
	}
}
