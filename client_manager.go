package main

import (
	"log"
	"sync"

	"github.com/gorilla/websocket"
)

// ClientManager handles WebSocket client connections
type ClientManager struct {
	clients    map[*websocket.Conn]bool
	mutex      sync.RWMutex
	register   chan *websocket.Conn
	unregister chan *websocket.Conn
	broadcast  chan Message
}

// NewClientManager creates a new client manager
func NewClientManager() *ClientManager {
	return &ClientManager{
		clients:    make(map[*websocket.Conn]bool),
		mutex:      sync.RWMutex{},
		register:   make(chan *websocket.Conn),
		unregister: make(chan *websocket.Conn),
		broadcast:  make(chan Message),
	}
}

// Start begins the client manager's operations
func (manager *ClientManager) Start() {
	for {
		select {
		case conn := <-manager.register:
			// Add new client
			manager.mutex.Lock()
			manager.clients[conn] = true
			manager.mutex.Unlock()
			log.Printf("New client connected (%d total)", len(manager.clients))
			
			// Send welcome message
			welcome := Message{
				Type:    "system",
				Message: "Connected to Minecraft chat bridge",
			}
			conn.WriteJSON(welcome)
			
		case conn := <-manager.unregister:
			// Remove client
			manager.mutex.Lock()
			if _, ok := manager.clients[conn]; ok {
				delete(manager.clients, conn)
				conn.Close()
			}
			manager.mutex.Unlock()
			log.Printf("Client disconnected (%d remaining)", len(manager.clients))
			
		case message := <-manager.broadcast:
			// Send message to all clients
			manager.SendToAll(message)
		}
	}
}

// SendToAll broadcasts a message to all connected clients
func (manager *ClientManager) SendToAll(message Message) {
	manager.mutex.RLock()
	defer manager.mutex.RUnlock()
	
	for conn := range manager.clients {
		go func(c *websocket.Conn, msg Message) {
			err := c.WriteJSON(msg)
			if err != nil {
				log.Printf("Error sending to client: %v", err)
				manager.unregister <- c
			}
		}(conn, message)
	}
}

// GetClientCount returns the number of connected clients
func (manager *ClientManager) GetClientCount() int {
	manager.mutex.RLock()
	defer manager.mutex.RUnlock()
	return len(manager.clients)
}
