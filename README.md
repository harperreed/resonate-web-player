# Resonate Player Go

A simple Go web application that receives Resonate audio streams and displays real-time metadata on a web interface.
<img width="748" height="715" alt="Screenshot 2025-10-26 at 10 39 05 PM" src="https://github.com/user-attachments/assets/b5a08bf4-46c4-4071-9aab-b78ab957163e" />

## Features

- Connects to a Resonate server as a receiver/player
- Displays now-playing metadata (artist, title, album, cover art) on a clean web interface
- Real-time updates via WebSocket
- Beautiful gradient UI with smooth animations and album artwork display
- Auto-reconnecting WebSocket client
- RESTful JSON API endpoint for metadata and player information

## Prerequisites

The following system dependencies are required:

### macOS
```bash
brew install pkg-config opus opusfile ffmpeg
```

### Ubuntu/Debian
```bash
sudo apt-get install pkg-config libopus-dev libopusfile-dev ffmpeg
```

### Fedora
```bash
sudo dnf install pkg-config opus-devel opusfile-devel ffmpeg
```

## Configuration

The application uses a YAML configuration file located at `config/config.yaml`:

```yaml
# Resonate server settings
server:
  address: "localhost:8927"        # Address of the Resonate server
  player_name: "Resonate Web Player"  # Name displayed in server

# Audio settings
audio:
  volume: 80  # Volume level (0-100)

# Web server settings
web:
  port: 8080       # Port for the web interface
  host: "0.0.0.0"  # Host to bind to
```

You can override the config path using the `CONFIG_PATH` environment variable:
```bash
CONFIG_PATH=/path/to/config.yaml ./resonate-player
```

## Building

### Local Build (Current Platform)

```bash
# Build for current platform
make build

# Or directly with go
go build -o resonate-player
```

### Linux Build (For Docker)

Since the application uses CGO for audio libraries, cross-compilation requires the target platform's headers. Use the Makefile to build the Linux binary in a Docker container:

```bash
# Build Linux binary (required for Docker on macOS/Windows)
make build-linux
```

This command:
- Runs a Docker container with Go 1.24 and all required audio libraries
- Builds the binary for Linux ARM64/AMD64
- Places the binary in the current directory

## Usage

### Running Locally

1. Ensure you have a Resonate server running (or update `config/config.yaml` with the server address)

2. Build and run:
```bash
make run
```

Or manually:
```bash
./resonate-player
```

3. Open your browser to:
```
http://localhost:8080
```

4. The web interface will display real-time metadata as the audio streams.

### Running with Docker

#### Quick Start with Make (Recommended)

The Makefile handles everything automatically:

```bash
# Build and start (builds Linux binary, Docker image, and starts container)
make docker-up

# View logs
make docker-logs

# Stop
make docker-down
```

#### Using Docker Compose

1. Build the Linux binary:
```bash
make build-linux
```

2. Start with docker-compose:
```bash
docker-compose up -d
```

3. View logs:
```bash
docker-compose logs -f
```

4. Stop the container:
```bash
docker-compose down
```

#### Using Docker Directly

1. Build everything:
```bash
make build-docker
```

Or manually:
```bash
make build-linux
docker build -f Dockerfile.simple -t resonate-player:latest .
```

2. Run the container:
```bash
docker run -d \
  --name resonate-player \
  -p 8080:8080 \
  -v $(pwd)/config:/app/config:ro \
  --network host \
  resonate-player:latest
```

**Note**: The `--network host` option is used to allow easy access to the Resonate server running on the host. On macOS/Windows, you may need to adjust the server address in `config/config.yaml` to use `host.docker.internal` instead of `localhost`.

#### Audio Device Configuration

For the container to play audio, it needs access to your host's audio system. The `docker-compose.yml` is pre-configured with both options:

**Option 1: PulseAudio (Recommended for most Linux systems)**

The default configuration shares your PulseAudio socket with the container:
```yaml
volumes:
  - /run/user/${UID:-1000}/pulse:/run/user/1000/pulse:ro
  - ~/.config/pulse/cookie:/root/.config/pulse/cookie:ro
environment:
  - PULSE_SERVER=unix:/run/user/1000/pulse/native
```

This works out-of-the-box on most modern Linux distributions using PulseAudio.

**Option 2: Direct ALSA Access**

If you're not using PulseAudio, the container can access ALSA devices directly:
```yaml
devices:
  - /dev/snd:/dev/snd
group_add:
  - audio
```

Both options are enabled in the default `docker-compose.yml`. If you have issues, try commenting out one approach.

**Testing Audio**

After starting the container, check if audio devices are accessible:
```bash
# Check ALSA devices
docker exec resonate-player ls -l /dev/snd

# Check PulseAudio connection (if using PulseAudio)
docker exec resonate-player pactl info
```

**Platform Notes**:
- **Linux**: Both PulseAudio and ALSA work natively
- **macOS/Windows**: Audio device passthrough is not supported with Docker Desktop. Consider:
  - Running the binary natively instead of in Docker
  - Using a Linux VM with device passthrough
  - Streaming audio over the network to another player

#### Customizing Configuration in Docker

You can mount a custom config file:
```bash
docker run -d \
  -p 8080:8080 \
  -v /path/to/your/config.yaml:/app/config/config.yaml:ro \
  resonate-player:latest
```

### Make Targets

Run `make help` to see all available targets:

```
make build         - Build for current platform
make build-linux   - Build Linux binary (in Docker)
make build-docker  - Build Docker image
make docker-up     - Start with docker-compose
make docker-down   - Stop docker-compose
make docker-logs   - View docker logs
make run           - Run locally
make clean         - Clean build artifacts
make help          - Show help
```

## API

### GET /metadata

Returns current player status and metadata as JSON.

**Example Response:**
```json
{
  "player": {
    "name": "Resonate Web Player",
    "server": "localhost:8927",
    "volume": 80,
    "connected_clients": 2
  },
  "current_track": {
    "title": "Song Title",
    "artist": "Artist Name",
    "album": "Album Name",
    "album_artist": "Album Artist",
    "artwork_url": "https://example.com/artwork.jpg",
    "track": 5,
    "year": 2024,
    "duration": 245
  }
}
```

**Usage:**
```bash
curl http://localhost:8080/metadata
```

This endpoint is useful for:
- Integrating with other applications
- Home automation systems
- Creating custom displays or notifications
- Monitoring player status

## Architecture

The application consists of:

- **Resonate Receiver**: Connects to the Resonate server and receives audio stream + metadata
- **HTTP Server**: Serves the web interface
- **WebSocket Server**: Broadcasts metadata updates to all connected web clients
- **Web UI**: Single-page application with auto-reconnecting WebSocket client

## Development

### Project Structure

```
.
├── main.go              # Main application with HTTP/WebSocket server and Resonate receiver
├── go.mod               # Go module definition
├── go.sum               # Go module checksums
├── config/
│   └── config.yaml      # Configuration file
├── Dockerfile           # Multi-stage Dockerfile (for building from source in Docker)
├── Dockerfile.simple    # Simple Dockerfile (uses pre-built binary)
├── docker-compose.yml   # Docker Compose configuration
├── .dockerignore        # Docker build exclusions
└── README.md            # This file
```

### Key Components

- `startResonateReceiver()`: Initializes and starts the Resonate player
- `handleMetadata()`: Called when new metadata is received from the stream
- `broadcastMetadata()`: Sends metadata updates (including cover art) to all connected WebSocket clients
- `handleWebSocket()`: Manages WebSocket connections
- `handleMetadataAPI()`: Serves JSON API endpoint with current player status and track metadata
- `handleIndex()`: Serves the HTML/CSS/JS web interface with album artwork display

## License

This project uses the [resonate-go](https://github.com/harperreed/resonate-go) library.

## Note

This is a proof of concept implementation. The Resonate protocol is still evolving and may change.
