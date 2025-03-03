// chat_server.go
package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
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
		// Add authentication as needed for your setup
		// ClientData: protocol.ClientData{},
	}

	// Connect to the Minecraft server
	// Replace with your server address
	conn, err := dialer.Dial("raknet", "localhost:19132")
	if err != nil {
		log.Fatalf("Error connecting to Minecraft: %v", err)
	}

	minecraftConn = conn
	log.Println("Connected to Minecraft server")

	// Handle incoming Minecraft packets
	go func() {
		defer conn.Close()
		
		for {
			pk, err := conn.ReadPacket()
			if err != nil {
				log.Printf("Error reading packet: %v", err)
				return
			}

			switch p := pk.(type) {
			case *packet.Text:
				// Handle incoming chat messages
				handleMinecraftChatMessage(p)
			}

			// Forward the packet to the Minecraft server
			if err := conn.WritePacket(pk); err != nil {
				log.Printf("Error forwarding packet: %v", err)
				return
			}
		}
	}()
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

	textPacket := &packet.Text{
		TextType: packet.TextTypeChat,
		Message:  message,
	}

	// If a target is specified, send as whisper
	if target != "" && target != "all" {
		textPacket.TextType = packet.TextTypeWhisper
		textPacket.TargetName = target
	}

	err := minecraftConn.WritePacket(textPacket)
	if err != nil {
		log.Printf("Error sending chat message: %v", err)
	}
}