package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

var rdb *redis.Client

func initRedis() {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		// Fallback for local testing if not set
		redisURL = "redis://localhost:6379/0"
	}

	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		log.Fatalf("Failed to parse REDIS_URL: %v", err)
	}

	rdb = redis.NewClient(opt)

	// Ping to ensure connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Printf("Warning: Failed to connect to Redis at %s. Please ensure Redis is running or use DEV_BYPASS. Error: %v", redisURL, err)
	} else {
		log.Println("Connected to Redis successfully.")
	}
}

// Room keys:
// room:{mid}:participants (Set) -> unique participant IDs (total population)
// room:{mid}:votes (Set) -> unique participant IDs who voted
// room:{mid}:triggered (String) -> '1' if triggered

const roomTTL = 24 * time.Hour

func AddParticipant(ctx context.Context, mid, uid string) error {
	pipe := rdb.Pipeline()
	partKey := fmt.Sprintf("room:%s:participants", mid)

	pipe.SAdd(ctx, partKey, uid)
	pipe.Expire(ctx, partKey, roomTTL)

	_, err := pipe.Exec(ctx)
	return err
}

func RemoveParticipant(ctx context.Context, mid, uid string) error {
	partKey := fmt.Sprintf("room:%s:participants", mid)
	return rdb.SRem(ctx, partKey, uid).Err()
}

func Vote(ctx context.Context, mid, uid string) (bool, error) {
	trigKey := fmt.Sprintf("room:%s:triggered", mid)
	isTriggered, err := rdb.Get(ctx, trigKey).Result()
	if err == nil && isTriggered == "1" {
		return false, nil // Already triggered, vote ignored
	}

	voteKey := fmt.Sprintf("room:%s:votes", mid)
	added, err := rdb.SAdd(ctx, voteKey, uid).Result()
	if err != nil {
		return false, err
	}
	rdb.Expire(ctx, voteKey, roomTTL)

	return added > 0, nil // True if it was a new vote
}

func CheckTriggerStatus(ctx context.Context, mid string) (int, int, bool, error) {
	partKey := fmt.Sprintf("room:%s:participants", mid)
	voteKey := fmt.Sprintf("room:%s:votes", mid)
	trigKey := fmt.Sprintf("room:%s:triggered", mid)

	// Fetch all state
	pipe := rdb.TxPipeline()
	totalCmd := pipe.SCard(ctx, partKey)
	votesCmd := pipe.SCard(ctx, voteKey)
	trigCmd := pipe.Get(ctx, trigKey)
	_, _ = pipe.Exec(ctx) // Ignoring exec error as missing keys return 0/redis.Nil

	total := int(totalCmd.Val())
	votes := int(votesCmd.Val())
	triggered := trigCmd.Val() == "1"

	if triggered {
		return total, votes, true, nil
	}

	if total > 0 {
		threshold := int(math.Ceil(float64(total) / 2.0))
		if votes >= threshold && votes > 0 {
			// Threshold met, mark as triggered
			rdb.Set(ctx, trigKey, "1", roomTTL)
			triggered = true
		}
	}

	return total, votes, triggered, nil
}
