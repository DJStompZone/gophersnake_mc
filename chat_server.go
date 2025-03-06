// chat_server.go
package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/login"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
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
	serverAddr := GetMinecraftServerAddress()
	log.Printf("Connecting to Minecraft server at %s...", serverAddr)

	// Use Python script for XBL3.0 authentication
	minecraftChain, privateKey, err := AuthenticateWithPythonXBL3()
	if err != nil {
		log.Printf("Error during authentication: %v", err)
		log.Println("Will retry connection in 10 seconds...")
		time.Sleep(10 * time.Second)
		connectMinecraft()
		return
	}

	dialer := minecraft.Dialer{
		ClientData: login.ClientData{
			GameVersion: GetGameVersion(),
			DeviceOS:    protocol.DeviceAndroid,
			DeviceID:    uuid.NewString(), // Generate a unique device ID
		},
		Protocol: minecraft.DefaultProtocol,
		IdentityData: login.IdentityData{
			Identity: minecraftChain,
			XUID:     "", // Will be filled by server
		},
	}

	// Store the private key for future use if needed
	_ = privateKey

	// Set PacketFunc for extra diagnostics
	dialer.PacketFunc = func(header packet.Header, data []byte, src, dst net.Addr) {
		log.Printf("Received packet: Header=%+v, DataLength=%d, Src=%v, Dst=%v", header, len(data), src, dst)
	}

	conn, err := dialer.Dial("raknet", serverAddr)
	if err != nil {
		log.Printf("Error connecting to Minecraft: %v", err)
		logMinecraftErrorDiagnostics(err)
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
