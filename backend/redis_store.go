package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"os"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	rdb      *redis.Client
	useRedis bool
	memRooms = sync.Map{} // map[string]*MemRoom
)

type MemRoom struct {
	mu           sync.RWMutex
	Participants map[string]bool
	Votes        map[string]bool
	Triggered    bool
}

func getMemRoom(mid string) *MemRoom {
	val, _ := memRooms.LoadOrStore(mid, &MemRoom{
		Participants: make(map[string]bool),
		Votes:        make(map[string]bool),
		Triggered:    false,
	})
	return val.(*MemRoom)
}

func initRedis() {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		log.Println("REDIS_URL not set. Falling back to in-memory store.")
		useRedis = false
		return
	}

	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		log.Printf("Failed to parse REDIS_URL: %v. Falling back to in-memory store.", err)
		useRedis = false
		return
	}

	rdb = redis.NewClient(opt)

	// Ping to ensure connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Printf("Warning: Failed to connect to Redis at %s. Falling back to in-memory store. Error: %v", redisURL, err)
		useRedis = false
		rdb.Close()
		rdb = nil
	} else {
		log.Println("Connected to Redis successfully.")
		useRedis = true
	}
}

const roomTTL = 24 * time.Hour

func AddParticipant(ctx context.Context, mid, uid string) error {
	if !useRedis {
		rm := getMemRoom(mid)
		rm.mu.Lock()
		rm.Participants[uid] = true
		rm.mu.Unlock()
		return nil
	}

	pipe := rdb.Pipeline()
	partKey := fmt.Sprintf("room:%s:participants", mid)

	pipe.SAdd(ctx, partKey, uid)
	pipe.Expire(ctx, partKey, roomTTL)

	_, err := pipe.Exec(ctx)
	return err
}

func RemoveParticipant(ctx context.Context, mid, uid string) error {
	if !useRedis {
		rm := getMemRoom(mid)
		rm.mu.Lock()
		delete(rm.Participants, uid)
		rm.mu.Unlock()
		return nil
	}

	partKey := fmt.Sprintf("room:%s:participants", mid)
	return rdb.SRem(ctx, partKey, uid).Err()
}

func Vote(ctx context.Context, mid, uid string) (bool, error) {
	if !useRedis {
		rm := getMemRoom(mid)
		rm.mu.Lock()
		defer rm.mu.Unlock()

		if rm.Triggered {
			return false, nil
		}
		if rm.Votes[uid] {
			return false, nil
		}
		rm.Votes[uid] = true
		return true, nil
	}

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
	if !useRedis {
		rm := getMemRoom(mid)
		rm.mu.Lock()
		defer rm.mu.Unlock()

		total := len(rm.Participants)
		votes := len(rm.Votes)

		if rm.Triggered {
			return total, votes, true, nil
		}

		if total > 0 {
			threshold := int(math.Ceil(float64(total) / 2.0))
			if votes >= threshold && votes > 0 {
				rm.Triggered = true
			}
		}

		return total, votes, rm.Triggered, nil
	}

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
