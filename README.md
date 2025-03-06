# GopherSnake MC

A bidirectional chat interface between Python and Minecraft Bedrock Edition dedicated servers using Go with gophertunnel.

## Overview

GopherSnake MC provides a bridge between Python applications and Minecraft Bedrock Edition's chat system. It consists of:

1. A Go server that connects to Minecraft using [gophertunnel](https://github.com/sandertv/gophertunnel)
2. A Python client library for easy integration with Python applications

This allows you to:

- Send chat messages to Minecraft from Python
- Receive chat messages from Minecraft in Python
- Create chatbots, automation tools, or external interfaces to Minecraft

## Requirements

### Go Server

- Go 1.18+
- [gorilla/websocket](https://github.com/gorilla/websocket)
- [sandertv/gophertunnel](https://github.com/sandertv/gophertunnel)
- [google/uuid](https://github.com/google/uuid)

### Python Client

- Python 3.6+
- [websocket-client](https://pypi.org/project/websocket-client/)
- For authentication: [msal](https://pypi.org/project/msal/) and [requests](https://pypi.org/project/requests/)

## Authentication

GopherSnake MC uses the Microsoft Xbox Live authentication system for online mode. This requires:

1. A valid Microsoft account with Minecraft purchased
2. Python with the required packages installed:

```bash
pip install msal requests
```

### Authentication Process

The authentication flow uses Microsoft Authentication Library (MSAL) to:

1. Authenticate with Microsoft Account using a device code flow
2. Exchange the Microsoft token for Xbox Live credentials
3. Get XSTS tokens necessary for Minecraft authentication
4. Cache tokens securely to avoid frequent re-authentication

You'll be prompted to visit a Microsoft URL and enter a code when authentication is needed.
Token caching ensures you won't need to do this every time you run the application.

### Authentication Troubleshooting

If you encounter authentication issues:

1. **Missing Python packages**: Ensure you've installed the required packages:
   ```bash
   pip install msal requests
   ```

2. **Token not being accepted**: Clear the cache by deleting the `xbl3_token_cache.bin` file (location will be shown in logs)

3. **Authentication flow fails**: Ensure you're completing the Microsoft login process within the time window (usually 15 minutes)

4. **Python not found**: Make sure Python is installed and in your PATH. The application tries to detect Python using different names (py, python, python3)

## Installation

1. Clone the repository:

   ```bash
   git clone https://github.com/DJStompZone/gophersnake_mc.git
   cd gophersnake_mc
   ```

2. Install Go dependencies:

   ```bash
   go mod download
   ```

3. Install Python dependencies:

   ```bash
   pip install websocket-client msal requests
   ```

## Configuration

Edit `config.json` to match your setup:

```jsonc
{
    "minecraft_server": {
        "address": "localhost",  // Your Bedrock server address
        "port": 19132,           // Your Bedrock server port (default: 19132)
        "online_mode": false,    // Set to true for Microsoft account authentication
        "game_version": "1.21.62" // Match your server version
    },
    "websocket": {
        "address": "0.0.0.0",    // WebSocket listen address (0.0.0.0 for all interfaces)
        "port": 8080             // WebSocket port
    },
    "player": {
        "display_name": "GopherSnake", // Name shown in Minecraft
        "device_id": "auto-generated"  // Leave as is for auto-generation
    }
}
```

## Running

### Start the Go Server

```bash
go run .
```

Or build and run the executable:

```bash
go build
./gophersnake_mc
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
```

## Troubleshooting

### Common Issues

1. **Connection refused to Minecraft server**
   - Ensure your Bedrock server is running and accessible
   - Check that the port in config.json matches your server's port
   - Try using the server's IP address instead of localhost

2. **Messages not being sent/received**
   - For dedicated servers, you may need to adjust permissions to allow chat commands
   - Check the server logs for any errors related to the connection
   - Try restarting both the Go server and the Minecraft server

3. **Authentication errors**
   - For online-mode servers, you'll need to set up proper authentication
   - Make sure you've installed the required Python packages: `pip install msal requests`
   - If problems persist, see the Authentication Troubleshooting section in this README

## Project Structure

```plaintext
gophersnake_mc/
├── main.go              # Main program entry point
├── chat_server.go       # Minecraft chat handling
├── client_manager.go    # WebSocket client management
├── config.go            # Configuration handling
├── imports.go           # Shared imports
├── config.json          # Configuration file
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