# GopherSnake MC

A bidirectional chat interface between Python and Minecraft Bedrock Edition using Go with gophertunnel.

**Note: This project is a work in progress and is not yet fully functional.**

## Overview

GopherSnake MC provides a bridge between Python applications and Minecraft Bedrock Edition's chat system. It consists of:

1. A Go server that connects to Minecraft using [gophertunnel](https://github.com/sandertv/gophertunnel)
2. A Python client library for easy integration with Python applications

This allows you to:

- Send chat messages to Minecraft from Python
- Receive chat messages from Minecraft in Python
- Create chatbots, automation tools, or external interfaces to Minecraft

## Architecture

```plaintext
+----------------+           +----------------+          +----------------+
|                | WebSocket |                |  packet  |                |
|   Python App   |<--------->|    Go Server   |<-------->|    Minecraft   |
|                |           |                |          |                |
+----------------+           +----------------+          +----------------+
```

## Requirements

### Go Server

- Go 1.18+
- [gorilla/websocket](https://github.com/gorilla/websocket)
- [sandertv/gophertunnel](https://github.com/sandertv/gophertunnel)

### Python Client

- Python 3.10+
- [websocket-client](https://pypi.org/project/websocket-client/)

## Installation

1. Clone the repository:

   ```bash
   git clone https://github.com/yourusername/gophersnake_mc.git
   cd gophersnake_mc
   ```

2. Install Go dependencies:

   ```bash
   go mod download
   ```

3. Install Python dependencies:

   ```bash
   pip install websocket-client
   ```

## Usage

### Starting the Go Server

1. Configure the server by editing `chat_server.go` to point to your Minecraft server:

   ```go
   // Replace with your server address
   conn, err := dialer.Dial("raknet", "localhost:19132")
   ```

2. Run the server:

   ```bash
   go run chat_server.go
   ```

### Using the Python Client

```python
from minecraft_chat import MinecraftChatClient

# Create a new client
client = MinecraftChatClient("ws://localhost:8080/chat")

# Register a callback for incoming chat messages
def on_chat_message(sender, message):
    print(f"[{sender}] {message}")
    
    # Auto-reply to messages containing "hello"
    if "hello" in message.lower():
        client.send_chat_message(f"Hello {sender}!")

client.register_chat_callback(on_chat_message)

# Connect to the server
client.connect()

# Send a message to all players
client.send_chat_message("Hello from Python!")

# Send a private message to a specific player
client.send_chat_message("This is a private message", target="PlayerName")
```

See the `examples/chat_example.py` for a complete example.

## Advanced Configuration

### Authentication

For servers that require authentication, you'll need to modify the `connectMinecraft` function in `chat_server.go`:

```go
func connectMinecraft() {
    dialer := minecraft.Dialer{
        ClientData: protocol.ClientData{
            GameVersion:      "1.20.0", // Set your game version
            DeviceOS:         device.Win10,
            DeviceModel:      "PC",
            ThirdPartyName:   "YourName",
            LanguageCode:     "en_US",
        },
    }
    
    // Add authentication token if needed for online servers
    // dialer.TokenSource = minecraft.NewTokenSource(...)
    
    // Connect to the server
    conn, err := dialer.Dial("raknet", "play.example.com:19132")
    // ...
}
```

Refer to the [gophertunnel documentation](https://pkg.go.dev/github.com/sandertv/gophertunnel) for more details on authentication.

## Project Structure

```plaintext
gophersnake_mc/
├── chat_server.go       # Go WebSocket server connecting to Minecraft
├── go.mod               # Go module definition
├── go.sum               # Go module checksums
├── minecraft_chat.py    # Python client library
└── examples/
    └── chat_example.py  # Example Python chat client
```

## License

MIT License - See LICENSE file for details

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request
