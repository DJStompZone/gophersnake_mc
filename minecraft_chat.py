# minecraft_chat.py
import websocket
import json
import threading
import time


class MinecraftChatClient:
    def __init__(self, server_url="ws://localhost:8080/chat"):
        self.server_url = server_url
        self.ws = None
        self.connected = False
        self._chat_callback = None
        self._connection_callback = None
        self._reconnect_attempts = 0
        self._max_reconnect_attempts = 5
        self._reconnect_delay = 2  # seconds
        self._running = False
        super().__init__()

    def connect(self, auto_reconnect=True):
        """Connect to the chat server with optional auto-reconnect"""
        self._running = True
        self._reconnect_attempts = 0
        self._try_connect(auto_reconnect)

    def _try_connect(self, auto_reconnect=True):
        """Internal method to establish connection"""
        if self.ws:
            self.ws.close()

        self.ws = websocket.WebSocketApp(
            self.server_url,
            on_message=self._on_message,
            on_error=self._on_error,
            on_close=self._on_close,
            on_open=self._on_open,
        )

        wst = threading.Thread(target=self.ws.run_forever)
        wst.daemon = True
        wst.start()

        # Wait briefly to check if connection succeeds
        time.sleep(1)

        # If auto-reconnect is enabled and we're not connected, try again
        if auto_reconnect and not self.connected and self._running:
            self._handle_reconnect()

    def _handle_reconnect(self):
        """Handle reconnection logic"""
        if self._reconnect_attempts >= self._max_reconnect_attempts:
            print(f"Failed to connect after {self._reconnect_attempts} attempts")
            self._running = False
            return

        self._reconnect_attempts += 1
        delay = self._reconnect_delay * self._reconnect_attempts
        print(f"Reconnecting in {delay} seconds (attempt {self._reconnect_attempts})")
        time.sleep(delay)

        if self._running:
            self._try_connect(True)

    def disconnect(self):
        """Disconnect from the chat server"""
        self._running = False
        if self.ws:
            self.ws.close()
        self.connected = False

    def send_chat_message(self, message, target=None):
        """Send a chat message to the Minecraft server"""
        if not self.connected:
            raise ConnectionError("Not connected to chat server")

        payload = {"type": "chat_message", "message": message}
        if target:
            payload["target"] = target

        if not self.ws:
            raise ConnectionError("Not connected to chat server")
        self.ws.send(json.dumps(payload))

    def register_chat_callback(self, callback):
        """Register a function to be called when a chat message is received

        The callback should accept (sender, message) parameters
        """
        self._chat_callback = callback

    def register_connection_callback(self, callback):
        """Register a function to be called when connection state changes

        The callback should accept a boolean parameter (connected)
        """
        self._connection_callback = callback

        # Call immediately with current state if we're already connected
        if self.connected and callback:
            callback(True)

    # Internal event handlers
    def _on_message(self, ws, message):
        try:
            data = json.loads(message)
            if data["type"] == "chat_message" and self._chat_callback:
                self._chat_callback(data["sender"], data["message"])
        except Exception as e:
            print(f"Error processing message: {e}")

    def _on_error(self, ws, error):
        print(f"WebSocket error: {error}")

    def _on_close(self, ws, close_status_code, close_msg):
        was_connected = self.connected
        self.connected = False

        if was_connected and self._connection_callback:
            self._connection_callback(False)

        print(f"Connection closed: {close_status_code} - {close_msg}")

        # Try to reconnect if we're still supposed to be running
        if self._running:
            self._handle_reconnect()

    def _on_open(self, ws):
        self.connected = True
        self._reconnect_attempts = 0
        print("Connection established")

        if self._connection_callback:
            self._connection_callback(True)
