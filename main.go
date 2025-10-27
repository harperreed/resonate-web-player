// ABOUTME: Simple Go web app that receives Resonate audio streams and displays metadata
// ABOUTME: Provides a web interface with real-time metadata updates via WebSocket

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/Resonate-Protocol/resonate-go/pkg/resonate"
	"github.com/gorilla/websocket"
	"gopkg.in/yaml.v3"
)

var (
	// Upgrader for WebSocket connections
	upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true // Allow all origins for development
		},
	}

	// Current metadata being broadcast
	currentMetadata resonate.Metadata
	metadataMutex   sync.RWMutex

	// WebSocket clients
	clients   = make(map[*websocket.Conn]bool)
	clientsMu sync.Mutex
)

// MetadataUpdate represents the JSON structure sent to clients
type MetadataUpdate struct {
	Artist     string `json:"artist"`
	Title      string `json:"title"`
	Album      string `json:"album,omitempty"`
	ArtworkURL string `json:"artwork_url,omitempty"`
}

// Config represents the application configuration
type Config struct {
	Server struct {
		Address    string `yaml:"address"`
		PlayerName string `yaml:"player_name"`
	} `yaml:"server"`
	Audio struct {
		Volume int `yaml:"volume"`
	} `yaml:"audio"`
	Web struct {
		Port int    `yaml:"port"`
		Host string `yaml:"host"`
	} `yaml:"web"`
}

var config Config

// loadConfig loads configuration from config.yaml
func loadConfig(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	if err := yaml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	return nil
}

func main() {
	// Load configuration
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "config/config.yaml"
	}

	if err := loadConfig(configPath); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	log.Printf("Loaded configuration from %s", configPath)
	// Start the Resonate receiver in a goroutine
	go startResonateReceiver()

	// HTTP handlers
	http.HandleFunc("/", handleIndex)
	http.HandleFunc("/ws", handleWebSocket)
	http.HandleFunc("/metadata", handleMetadataAPI)

	addr := fmt.Sprintf("%s:%d", config.Web.Host, config.Web.Port)
	log.Printf("Starting web server on http://%s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatal(err)
	}
}

// startResonateReceiver initializes and starts the Resonate player
func startResonateReceiver() {
	log.Printf("Connecting to Resonate server at %s as '%s'", config.Server.Address, config.Server.PlayerName)
	player, err := resonate.NewPlayer(resonate.PlayerConfig{
		ServerAddr: config.Server.Address,
		PlayerName: config.Server.PlayerName,
		Volume:     config.Audio.Volume,
		OnMetadata: handleMetadata,
	})
	if err != nil {
		log.Printf("Error creating player: %v", err)
		return
	}

	log.Println("Connecting to Resonate server...")
	if err := player.Connect(); err != nil {
		log.Printf("Error connecting to server: %v", err)
		return
	}

	log.Println("Starting playback...")
	if err := player.Play(); err != nil {
		log.Printf("Error starting playback: %v", err)
		return
	}

	log.Println("Resonate receiver started successfully")
	select {} // Keep running
}

// handleMetadata is called when new metadata is received from the Resonate stream
func handleMetadata(meta resonate.Metadata) {
	metadataMutex.Lock()
	currentMetadata = meta
	metadataMutex.Unlock()

	log.Printf("Now playing: %s - %s", meta.Artist, meta.Title)

	// Broadcast to all connected WebSocket clients
	broadcastMetadata(meta)
}

// broadcastMetadata sends metadata updates to all connected WebSocket clients
func broadcastMetadata(meta resonate.Metadata) {
	update := MetadataUpdate{
		Artist:     meta.Artist,
		Title:      meta.Title,
		Album:      meta.Album,
		ArtworkURL: meta.ArtworkURL,
	}

	message, err := json.Marshal(update)
	if err != nil {
		log.Printf("Error marshaling metadata: %v", err)
		return
	}

	clientsMu.Lock()
	defer clientsMu.Unlock()

	for client := range clients {
		err := client.WriteMessage(websocket.TextMessage, message)
		if err != nil {
			log.Printf("Error sending to client: %v", err)
			client.Close()
			delete(clients, client)
		}
	}
}

// handleWebSocket handles WebSocket connections for real-time updates
func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	clientsMu.Lock()
	clients[conn] = true
	clientsMu.Unlock()

	log.Printf("New WebSocket client connected (total: %d)", len(clients))

	// Send current metadata immediately
	metadataMutex.RLock()
	if currentMetadata.Title != "" {
		update := MetadataUpdate{
			Artist:     currentMetadata.Artist,
			Title:      currentMetadata.Title,
			Album:      currentMetadata.Album,
			ArtworkURL: currentMetadata.ArtworkURL,
		}
		metadataMutex.RUnlock()

		message, _ := json.Marshal(update)
		conn.WriteMessage(websocket.TextMessage, message)
	} else {
		metadataMutex.RUnlock()
	}

	// Keep connection alive and handle disconnection
	defer func() {
		clientsMu.Lock()
		delete(clients, conn)
		clientsMu.Unlock()
		conn.Close()
		log.Printf("WebSocket client disconnected (total: %d)", len(clients))
	}()

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

// handleMetadataAPI serves the current metadata and player information as JSON
func handleMetadataAPI(w http.ResponseWriter, r *http.Request) {
	metadataMutex.RLock()
	metadata := currentMetadata
	metadataMutex.RUnlock()

	clientsMu.Lock()
	connectedClients := len(clients)
	clientsMu.Unlock()

	response := map[string]interface{}{
		"player": map[string]interface{}{
			"name":              config.Server.PlayerName,
			"server":            config.Server.Address,
			"volume":            config.Audio.Volume,
			"connected_clients": connectedClients,
		},
		"current_track": map[string]interface{}{
			"title":        metadata.Title,
			"artist":       metadata.Artist,
			"album":        metadata.Album,
			"album_artist": metadata.AlbumArtist,
			"artwork_url":  metadata.ArtworkURL,
			"track":        metadata.Track,
			"year":         metadata.Year,
			"duration":     metadata.Duration,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleIndex serves the main HTML page
func handleIndex(w http.ResponseWriter, r *http.Request) {
	html := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Resonate Web Player</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }

        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, sans-serif;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
            padding: 20px;
        }

        .player-container {
            background: rgba(255, 255, 255, 0.95);
            border-radius: 20px;
            padding: 40px;
            box-shadow: 0 20px 60px rgba(0, 0, 0, 0.3);
            max-width: 600px;
            width: 100%;
            backdrop-filter: blur(10px);
        }

        h1 {
            text-align: center;
            color: #333;
            margin-bottom: 30px;
            font-size: 2em;
        }

        .metadata-display {
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            border-radius: 15px;
            padding: 40px;
            color: white;
            text-align: center;
            min-height: 200px;
            display: flex;
            flex-direction: column;
            align-items: center;
            justify-content: center;
            transition: all 0.3s ease;
            gap: 20px;
        }

        .artwork-container {
            width: 200px;
            height: 200px;
            border-radius: 10px;
            overflow: hidden;
            box-shadow: 0 10px 30px rgba(0, 0, 0, 0.3);
            display: none;
        }

        .artwork-container.visible {
            display: block;
        }

        .artwork-container img {
            width: 100%;
            height: 100%;
            object-fit: cover;
        }

        .metadata-text {
            display: flex;
            flex-direction: column;
            gap: 10px;
        }

        .song-title {
            font-size: 2em;
            font-weight: bold;
            word-wrap: break-word;
        }

        .artist-name {
            font-size: 1.5em;
            opacity: 0.9;
        }

        .album-name {
            font-size: 1em;
            opacity: 0.7;
        }

        .status {
            text-align: center;
            margin-top: 20px;
            padding: 10px;
            border-radius: 5px;
            font-size: 0.9em;
        }

        .status.connected {
            background: #d4edda;
            color: #155724;
        }

        .status.disconnected {
            background: #f8d7da;
            color: #721c24;
        }

        .status.waiting {
            background: #fff3cd;
            color: #856404;
        }

        @keyframes pulse {
            0%, 100% { opacity: 1; }
            50% { opacity: 0.5; }
        }

        .waiting-animation {
            animation: pulse 2s ease-in-out infinite;
        }
    </style>
</head>
<body>
    <div class="player-container">
        <h1>🎵 Resonate Player</h1>

        <div class="metadata-display" id="metadata">
            <div class="artwork-container" id="artwork">
                <img src="" alt="Album artwork" id="artworkImg">
            </div>
            <div class="metadata-text">
                <div class="song-title waiting-animation">Waiting for stream...</div>
                <div class="artist-name">—</div>
            </div>
        </div>

        <div class="status waiting" id="status">Connecting to server...</div>
    </div>

    <script>
        let ws;
        let reconnectInterval = 3000;
        const statusEl = document.getElementById('status');
        const metadataEl = document.getElementById('metadata');

        function connect() {
            const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
            const wsUrl = protocol + '//' + window.location.host + '/ws';

            ws = new WebSocket(wsUrl);

            ws.onopen = function() {
                console.log('WebSocket connected');
                statusEl.textContent = 'Connected ✓';
                statusEl.className = 'status connected';
            };

            ws.onmessage = function(event) {
                const data = JSON.parse(event.data);
                updateMetadata(data);
            };

            ws.onerror = function(error) {
                console.error('WebSocket error:', error);
            };

            ws.onclose = function() {
                console.log('WebSocket disconnected');
                statusEl.textContent = 'Disconnected - Reconnecting...';
                statusEl.className = 'status disconnected';
                setTimeout(connect, reconnectInterval);
            };
        }

        function updateMetadata(data) {
            const titleEl = metadataEl.querySelector('.song-title');
            const artistEl = metadataEl.querySelector('.artist-name');
            const metadataText = metadataEl.querySelector('.metadata-text');
            let albumEl = metadataEl.querySelector('.album-name');
            const artworkContainer = document.getElementById('artwork');
            const artworkImg = document.getElementById('artworkImg');

            titleEl.textContent = data.title || 'Unknown Title';
            titleEl.classList.remove('waiting-animation');
            artistEl.textContent = data.artist || 'Unknown Artist';

            // Handle album name
            if (data.album) {
                if (!albumEl) {
                    albumEl = document.createElement('div');
                    albumEl.className = 'album-name';
                    metadataText.appendChild(albumEl);
                }
                albumEl.textContent = data.album;
            } else if (albumEl) {
                albumEl.remove();
            }

            // Handle artwork
            if (data.artwork_url) {
                artworkImg.src = data.artwork_url;
                artworkContainer.classList.add('visible');
            } else {
                artworkImg.src = '';
                artworkContainer.classList.remove('visible');
            }

            statusEl.textContent = 'Now Playing ♫';
            statusEl.className = 'status connected';
        }

        // Start connection
        connect();
    </script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, html)
}
