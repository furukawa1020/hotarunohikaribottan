package main

import (
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // allowing any origin for testing
	},
}

type Client struct {
	conn   *websocket.Conn
	roomID string
	pid    string
}

// Global clients map
var clients = make(map[*Client]bool)

func handleConnections(w http.ResponseWriter, r *http.Request) {
	roomID := r.URL.Query().Get("roomId")
	pid := r.URL.Query().Get("pid")
	if roomID == "" || pid == "" {
		http.Error(w, "missing roomId or pid query param", http.StatusBadRequest)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade Error:", err)
		return
	}
	defer conn.Close()

	client := &Client{conn: conn, roomID: roomID, pid: pid}
	clients[client] = true
	defer delete(clients, client)

	room := getRoom(roomID)
	room.AddParticipant(pid)
	defer room.RemoveParticipant(pid)

	// Broadcast updated gauge on join
	if !room.IsTriggered {
		broadcastRoom(roomID, room.GenerateGaugeHTML())
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
			if room.Vote(pid) {
				if room.CheckTrigger() {
					broadcastRoom(roomID, GenerateTriggeredHTML())
				} else {
					broadcastRoom(roomID, room.GenerateGaugeHTML())
				}
			}
		}
	}

	// On disconnect, update gauge for others remaining
	// Delay is minimal; in production we might debounce this.
	// We handle it simply here.
	if !room.IsTriggered {
		broadcastRoom(roomID, room.GenerateGaugeHTML())
	}
}

func isVoteMessage(msg map[string]interface{}) bool {
	headers, ok := msg["HEADERS"].(map[string]interface{})
	if !ok {
		return false
	}
	if v, exists := headers["HX-Request"]; exists && v == "true" {
		return true // It's an HTMX requested message (clicked '帰る')
	}
	return false
}

func broadcastRoom(roomID string, html string) {
	for client := range clients {
		if client.roomID == roomID {
			err := client.conn.WriteMessage(websocket.TextMessage, []byte(html))
			if err != nil {
				log.Printf("WS write error: %v", err)
				client.conn.Close()
				delete(clients, client)
			}
		}
	}
}

func main() {
	fs := http.FileServer(http.Dir("../frontend"))
	http.Handle("/", fs)
	http.HandleFunc("/ws", handleConnections)

	log.Println("Go Server started on http://localhost:8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal("ListenAndServe:", err)
	}
}
