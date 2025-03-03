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
	serverAddr := GetWebSocketAddress() // Use configuration if available, otherwise use default
	log.Printf("Starting WebSocket server on %s", serverAddr)
	err := http.ListenAndServe(serverAddr, nil)
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
	serverAddr := GetMinecraftServerAddress()

	dialer := minecraft.Dialer{
		ClientData: login.ClientData{
			GameVersion: GetGameVersion(),
			DeviceOS:    protocol.DeviceAndroid,
		},
		Protocol: minecraft.DefaultProtocol,
	}

	// NEW: Set PacketFunc for extra diagnostics.
	dialer.PacketFunc = func(header packet.Header, data []byte, src, dst net.Addr) {
		log.Printf("Received packet: Header=%+v, DataLength=%d, Src=%v, Dst=%v", header, len(data), src, dst)
	}

	log.Printf("Connecting to Minecraft server at %s...", serverAddr)

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
			TextType:         packet.TextTypeChat,
			NeedsTranslation: false,
			SourceName:       "gophersnake",
			Message:          message,
			Parameters:       []string{},
		}

		// Add target for whispers
		if target != "" && target != "all" {
			textPacket.TextType = packet.TextTypeWhisper
			// Fix: TextPacket doesn't have TargetName field in newer gophertunnel
			// Add the target to the Parameters instead
			textPacket.Parameters = append(textPacket.Parameters, target)
		}

		err = minecraftConn.WritePacket(textPacket)
		if err != nil {
			log.Printf("Error sending text packet: %v", err)
		}
	}
}
