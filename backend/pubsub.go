package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
)

// The channel prefix for broadcasting messages
const pubSubChannelPrefix = "room-broadcast:"

// PubSubMessage defines the payload structure sent across Redis
type PubSubMessage struct {
	RoomID string `json:"roomId"`
	Event  string `json:"event"` // 'update' or 'triggered'
	HTML   string `json:"html"`  // HTML fragment to replace via HTMX
}

// PublishRoomUpdate sends a request to all servers to update the gauge for this room
func PublishRoomUpdate(ctx context.Context, mid string) {
	html := GenerateGaugeFromDB(ctx, mid)
	if html == "" {
		return
	}

	msg := PubSubMessage{
		RoomID: mid,
		Event:  "update",
		HTML:   html,
	}

	publish(ctx, mid, msg)
}

// PublishRoomUpdateTriggered sends the ENDING screen HTML to all servers for this room
func PublishRoomUpdateTriggered(ctx context.Context, mid string) {
	html := GenerateTriggeredHTML()
	msg := PubSubMessage{
		RoomID: mid,
		Event:  "triggered",
		HTML:   html,
	}

	publish(ctx, mid, msg)
}

func publish(ctx context.Context, mid string, msg PubSubMessage) {
	channel := fmt.Sprintf("%s%s", pubSubChannelPrefix, mid)
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Failed to marshal PubSubMessage: %v", err)
		return
	}

	err = rdb.Publish(ctx, channel, data).Err()
	if err != nil {
		log.Printf("Failed to publish to channel %s: %v", channel, err)
	}
}

// ListenPubSub subscribes to Redis channels using Pattern Subscription
// and forwards received HTML fragments to local WebSocket clients.
func ListenPubSub(ctx context.Context) {
	// PSubscribe listens to all channels starting with pubSubChannelPrefix
	pattern := fmt.Sprintf("%s*", pubSubChannelPrefix)
	pubsub := rdb.PSubscribe(ctx, pattern)
	defer pubsub.Close()

	ch := pubsub.Channel()
	log.Printf("Listening to Redis PubSub pattern: %s", pattern)

	for msg := range ch {
		var psMsg PubSubMessage
		if err := json.Unmarshal([]byte(msg.Payload), &psMsg); err != nil {
			log.Printf("Failed to unmarshal received PubSub msg: %v", err)
			continue
		}

		// Push the HTML directly to all locally connected websockets in the room
		broadcastLocalRoom(psMsg.RoomID, psMsg.HTML)
	}
}
