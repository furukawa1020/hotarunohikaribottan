package main

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func setupTestRedis() (*miniredis.Miniredis, *redis.Client) {
	mr, err := miniredis.Run()
	if err != nil {
		panic(err)
	}

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	useRedis = true // Ensure tests use the Redis logic path
	return mr, client
}

func TestVoteAndTriggerThreshold(t *testing.T) {
	mr, client := setupTestRedis()
	defer mr.Close()

	// Override the global var for testing
	rdb = client
	ctx := context.Background()
	roomID := "testRoom1"

	// 1. Add 3 Participants
	AddParticipant(ctx, roomID, "user1")
	AddParticipant(ctx, roomID, "user2")
	AddParticipant(ctx, roomID, "user3")

	total, votes, triggered, err := CheckTriggerStatus(ctx, roomID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 3 || votes != 0 || triggered != false {
		t.Errorf("expected 3/0/false, got %d/%d/%t", total, votes, triggered)
	}

	// 2. Vote User 1 -> 1/3 (Not Triggered)
	added, err := Vote(ctx, roomID, "user1")
	if !added || err != nil {
		t.Errorf("expected true vote, got %t %v", added, err)
	}

	total, votes, triggered, _ = CheckTriggerStatus(ctx, roomID)
	if total != 3 || votes != 1 || triggered != false {
		t.Errorf("expected 3/1/false, got %d/%d/%t", total, votes, triggered)
	}

	// 3. Double Vote User 1 -> Ignored
	added, _ = Vote(ctx, roomID, "user1")
	if added {
		t.Errorf("expected double vote to return false, got true")
	}

	// 4. Vote User 2 -> 2/3 (Should Trigger since 2 >= math.Ceil(3/2)=2)
	added, _ = Vote(ctx, roomID, "user2")
	if !added {
		t.Errorf("expected second vote to return true")
	}

	total, votes, triggered, _ = CheckTriggerStatus(ctx, roomID)
	if total != 3 || votes != 2 || triggered != true {
		t.Errorf("expected 3/2/true (Triggered!), got %d/%d/%t", total, votes, triggered)
	}

	// 5. Vote User 3 -> Should be ignored as already triggered
	added, _ = Vote(ctx, roomID, "user3")
	if added {
		t.Errorf("expected vote after trigger to return false")
	}
}

func TestTTLSet(t *testing.T) {
	mr, client := setupTestRedis()
	defer mr.Close()

	rdb = client
	ctx := context.Background()
	roomID := "testRoom2"

	AddParticipant(ctx, roomID, "u1")
	Vote(ctx, roomID, "u1")

	mr.FastForward(25 * time.Hour)

	total, votes, _, _ := CheckTriggerStatus(ctx, roomID)
	if total != 0 || votes != 0 {
		t.Errorf("Data did not expire after 24h: got total %d, votes %d", total, votes)
	}
}
