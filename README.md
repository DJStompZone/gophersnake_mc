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

## Configuration

Edit `config.json` to match your setup:
