// chat_server.go
package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sandertv/gophertunnel/minecraft/protocol/device"
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
	// Start the Minecraft connection manager
	go connectMinecraft()

	// Setup WebSocket route
	http.HandleFunc("/chat", handleConnections)

	// Start HTTP server
	log.Println("Starting WebSocket server on :8080")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("Error starting HTTP server:", err)
	}
}

func handleConnections(w http.ResponseWriter, r *http.Request) {
	// Upgrade initial GET request to a WebSocket
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Error upgrading connection: %v", err)
		return
	}
	defer ws.Close()

	// Register new client
	clientsMux.Lock()
	clients[ws] = true
	clientsMux.Unlock()

	log.Println("New Python client connected")

	// Handle WebSocket messages
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

		// Process the message
		if msg.Type == "chat_message" && minecraftConn != nil {
			sendChatToMinecraft(msg.Message, msg.Target)
		}
	}
}

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

func connectMinecraft() {
	// Create a dialer to connect to a Minecraft server
	dialer := minecraft.Dialer{
		ClientData: protocol.ClientData{
			GameVersion:       "1.21.62",             // Update this to match your server version
			DeviceOS:          device.DeviceOS(1),   // Windows (DeviceOS 1)
			DeviceID:          uuid.New().String(),  // Generate a unique device ID
			DeviceModel:       "gophersnake_client", // Custom device model name
			ThirdPartyName:    "gophersnake",        // Display name in-game
			IdentityPublicKey: "gophersnake",        // Required for newer versions
			SkinID:            uuid.New().String(),  // Random skin ID
			LanguageCode:      "en_US",              // Language setting
		},
		// For offline mode servers, we don't need to set a TokenSource
	}

	// Connect to the Minecraft server
	// Replace with your server address (standard port is 19132)
	serverAddr := "localhost:19132"
	log.Printf("Connecting to Minecraft server at %s...", serverAddr)
	
	conn, err := dialer.Dial("raknet", serverAddr)
	if err != nil {
		log.Printf("Error connecting to Minecraft: %v", err)
		log.Println("Will retry connection in 10 seconds...")
		
		// Try to reconnect after a delay
		time.Sleep(10 * time.Second)
		connectMinecraft()
		return
	}

	minecraftConn = conn
	log.Println("Connected to Minecraft server successfully!")

	// Handle incoming Minecraft packets
	go handleMinecraftConnection(conn)
}

func handleMinecraftConnection(conn *minecraft.Conn) {
	defer func() {
		conn.Close()
		minecraftConn = nil
		log.Println("Minecraft connection closed, attempting to reconnect...")
		
		// Try to reconnect after a delay
		time.Sleep(5 * time.Second)
		connectMinecraft()
	}()
	
	for {
		// Read the next packet from the connection
		pk, err := conn.ReadPacket()
		if err != nil {
			log.Printf("Error reading packet: %v", err)
			return
		}

		// Handle different packet types
		switch p := pk.(type) {
		case *packet.Text:
			// Process chat messages
			handleMinecraftChatMessage(p)
		case *packet.Disconnect:
			// Server kicked us
			log.Printf("Disconnected by server: %s", p.Message)
			return
		}

		// Forward the packet to maintain the connection
		if err := conn.WritePacket(pk); err != nil {
			log.Printf("Error forwarding packet: %v", err)
			return
		}
	}
}

func handleMinecraftChatMessage(textPacket *packet.Text) {
	// Skip system messages and commands
	if textPacket.TextType != packet.TextTypeChat && textPacket.TextType != packet.TextTypeWhisper {
		return
	}

	// Create message object
	msg := Message{
		Type:    "chat_message",
		Message: textPacket.Message,
		Sender:  textPacket.SourceName,
	}

	// Broadcast to all WebSocket clients
	broadcastToWebsocket(msg)
}

func sendChatToMinecraft(message string, target string) {
	if minecraftConn == nil {
		log.Println("Not connected to Minecraft server")
		return
	}

	// Different approach for sending messages on a dedicated server
	// For dedicated servers, we typically send commands rather than text packets
	
	var commandStr string
	if target != "" && target != "all" {
		// Send as a whisper using /tell command
		commandStr = fmt.Sprintf("/tell %s %s", target, message)
	} else {
		// Send as regular chat (on dedicated servers, this is often a /say command)
		// But we can also try with just the message for player chat
		commandStr = message
		
		// Alternative: use /say for broadcasts
		// commandStr = fmt.Sprintf("/say %s", message)
	}
	
	log.Printf("Sending to Minecraft: %s", commandStr)
	
	// Create a command request packet
	cmdPacket := &packet.CommandRequest{
		CommandLine: commandStr,
		CommandOrigin: protocol.CommandOrigin{
			Origin:    protocol.CommandOriginPlayer,
			UUID:      uuid.New(),
			RequestID: strings.ReplaceAll(uuid.New().String(), "-", ""),
		},
	}
	
	// Try sending as a command first
	err := minecraftConn.WritePacket(cmdPacket)
	if err != nil {
		log.Printf("Error sending command: %v", err)
		
		// Fall back to text packet if command fails
		textPacket := &packet.Text{
			TextType:     packet.TextTypeChat,
			NeedsTranslation: false,
			SourceName:   "gophersnake",
			Message:      message,
			Parameters:   []string{},
		}
		
		// Add target for whispers
		if target != "" && target != "all" {
			textPacket.TextType = packet.TextTypeWhisper
			textPacket.TargetName = target
		}
		
		err = minecraftConn.WritePacket(textPacket)
		if err != nil {
			log.Printf("Error sending text packet: %v", err)
		}
	}
}