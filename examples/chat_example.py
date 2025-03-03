import sys
import time

sys.path.append("..")  # Add parent directory to path

from minecraft_chat import MinecraftChatClient


def on_chat_message(sender, message):
    print(f"[{sender}] {message}")


def on_connection_change(connected):
    if connected:
        print("Connected to Minecraft chat!")
    else:
        print("Disconnected from Minecraft chat")


def main():
    # Create chat client
    client = MinecraftChatClient()

    # Register callbacks
    client.register_chat_callback(on_chat_message)
    client.register_connection_callback(on_connection_change)

    # Connect to server
    client.connect()

    try:
        # Wait for connection
        time.sleep(2)

        if not client.connected:
            print("Could not connect to chat server. Make sure it's running.")
            return

        # Send a message
        client.send_chat_message("Hello from Python!")

        # Keep the script running to receive messages
        print("Listening for chat messages. Press Ctrl+C to exit.")
        while True:
            message = input("> ")
            if message.lower() == "exit":
                break
            client.send_chat_message(message)
    except KeyboardInterrupt:
        print("Exiting...")
    finally:
        client.disconnect()


if __name__ == "__main__":
    main()
