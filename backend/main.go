package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// Allows integration from any origin (Zoom Client, Localhost ngrok, etc.)
		return true
	},
}

type Client struct {
	conn   *websocket.Conn
	roomID string
	pid    string
}

// In a real multi-server cluster, clients map only holds local connections.
// Broadcasts to other servers happen via Redis PubSub.
var (
	clients   = make(map[*Client]bool)
	clientsMu sync.RWMutex
)

func handleConnections(w http.ResponseWriter, r *http.Request) {
	// 1. Retrieve Context from AuthMiddleware
	val := r.Context().Value("zoomCtx")
	if val == nil {
		http.Error(w, "Unauthorized Context Missing", http.StatusUnauthorized)
		return
	}
	zoomCtx, ok := val.(*ZoomAuthContext)
	if !ok {
		http.Error(w, "Invalid Context Type", http.StatusInternalServerError)
		return
	}

	roomID := zoomCtx.Mid
	pid := zoomCtx.UID

	if roomID == "" || pid == "" {
		http.Error(w, "missing roomId or pid from Context", http.StatusBadRequest)
		return
	}

	// 2. Upgrade HTTP to WS
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade Error:", err)
		return
	}
	defer conn.Close()

	client := &Client{conn: conn, roomID: roomID, pid: pid}

	clientsMu.Lock()
	clients[client] = true
	clientsMu.Unlock()

	defer func() {
		clientsMu.Lock()
		delete(clients, client)
		clientsMu.Unlock()
	}()

	// Context for Redis ops
	ctx := context.Background()

	// 3. Add to Redis Participants
	if err := AddParticipant(ctx, roomID, pid); err != nil {
		log.Printf("Redis AddParticipant Error: %v", err)
	}

	// Broadcast updated gauge on join (via Local & PubSub)
	_, _, isTriggered, _ := CheckTriggerStatus(ctx, roomID)
	if !isTriggered {
		PublishRoomUpdate(ctx, roomID)
	} else {
		// New participant joining triggered room, send them ending screen directly
		conn.WriteMessage(websocket.TextMessage, []byte(GenerateTriggeredHTML()))
	}

	for {
		var msg map[string]interface{}
		err := conn.ReadJSON(&msg)
		if err != nil {
			log.Printf("error reading WS JSON: %v", err)
			break
		}

		if isVoteMessage(msg) {
			added, err := Vote(ctx, roomID, pid)
			if err != nil {
				log.Printf("Vote error: %v", err)
				continue
			}

			if added { // Only process if it was a new vote
				_, _, triggered, err := CheckTriggerStatus(ctx, roomID)
				if err != nil {
					log.Printf("CheckTrigger error: %v", err)
				}

				if triggered {
					PublishRoomUpdateTriggered(ctx, roomID)
				} else {
					PublishRoomUpdate(ctx, roomID)
				}
			}
		}
	}

	// On disconnect
	RemoveParticipant(ctx, roomID, pid)
	_, _, triggered, _ := CheckTriggerStatus(ctx, roomID)
	if !triggered {
		PublishRoomUpdate(ctx, roomID)
	}
}

func isVoteMessage(msg map[string]interface{}) bool {
	headers, ok := msg["HEADERS"].(map[string]interface{})
	if !ok {
		return false
	}
	if v, exists := headers["HX-Request"]; exists && v == "true" {
		return true // clicked '帰る'
	}
	return false
}

// Broadcasts locally to all connected sockets for this room
func broadcastLocalRoom(roomID string, html string) {
	clientsMu.RLock()
	defer clientsMu.RUnlock()

	for client := range clients {
		if client.roomID == roomID {
			err := client.conn.WriteMessage(websocket.TextMessage, []byte(html))
			if err != nil {
				log.Printf("WS write error: %v", err)
				client.conn.Close()
				// We do not delete here while holding RLock.
				// The defer block in handleConnections will clean it up when ReadJSON fails.
			}
		}
	}
}

// Generate gauge string based on Redis DB values
func GenerateGaugeFromDB(ctx context.Context, mid string) string {
	total, votes, triggered, err := CheckTriggerStatus(ctx, mid)
	if err != nil {
		return ""
	}
	if triggered {
		return ""
	}

	percent := 0.0
	if total > 0 {
		percent = float64(votes) / float64(total) * 100
	}

	statusText := "まだ早い"
	if percent >= 50 {
		statusText = "帰宅"
	} else if percent >= 26 {
		statusText = "そろそろ…"
	}

	html := fmt.Sprintf(`
<div id="gauge-container" hx-swap-oob="true">
	<div class="gauge">
		<div class="gauge-fill" style="width: %.1f%%;"></div>
	</div>
	<p class="status-text">%s <span class="anonym-info">(匿名)</span></p>
</div>
`, percent, statusText)
	return html
}

func GenerateTriggeredHTML() string {
	return `
<div id="main-ui" hx-swap-oob="true" class="triggered-mode">
	<div class="ending-screen">
		<h1 class="ending-title">本日の営業は終了しました</h1>
		<p class="ending-sub">速やかにご退出ください</p>
		<audio autoplay loop src="/hotaru-piano.mp3"></audio>
	</div>
</div>
`
}

func main() {
	// Initialize Redis Connection
	initRedis()
	defer func() {
		if rdb != nil {
			rdb.Close()
			log.Println("Redis connection closed")
		}
	}()

	// Initialize PubSub Listener
	pubSubCtx, pubSubCancel := context.WithCancel(context.Background())
	defer pubSubCancel()
	go ListenPubSub(pubSubCtx)

	fs := http.FileServer(http.Dir("../frontend"))
	mux := http.NewServeMux()
	mux.Handle("/", fs)

	// Apply Auth Middleware to WS endpoint with Rate Limiting logic implicitly handled by HMAC state
	mux.HandleFunc("/ws", AuthMiddleware(handleConnections))

	port := strings.TrimSpace(os.Getenv("PORT"))
	if port == "" {
		port = "8080"
	}

	server := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	// Graceful Shutdown Channel
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Println("Robust Go Server started on port " + port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("ListenAndServe error: %v", err)
		}
	}()

	<-stop // Block until signal
	log.Println("Shutting down gracefully...")

	// Create a deadline for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server force shutdown: %v", err)
	}

	log.Println("Server stopping successfully")
}
