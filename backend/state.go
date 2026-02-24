package main

import (
	"fmt"
	"math"
	"sync"
)

type Room struct {
	mu             sync.Mutex
	Participants   map[string]bool // participantId -> active
	VotedUsers     map[string]bool // participantId -> voted
	IsTriggered    bool
}

func NewRoom() *Room {
	return &Room{
		Participants: make(map[string]bool),
		VotedUsers:   make(map[string]bool),
	}
}

// Global rooms map
var rooms = struct {
	sync.RWMutex
	m map[string]*Room
}{m: make(map[string]*Room)}

func getRoom(id string) *Room {
	rooms.RLock()
	r, ok := rooms.m[id]
	rooms.RUnlock()

	if !ok {
		rooms.Lock()
		r, ok = rooms.m[id]
		if !ok {
			r = NewRoom()
			rooms.m[id] = r
		}
		rooms.Unlock()
	}
	return r
}

func (r *Room) AddParticipant(pid string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Participants[pid] = true
}

func (r *Room) RemoveParticipant(pid string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.Participants, pid) // 実際の運用ではコネクション切れを厳密に管理
}

func (r *Room) Vote(pid string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.IsTriggered {
		return false // already triggered
	}

	if r.VotedUsers[pid] {
		return false // already voted
	}

	r.VotedUsers[pid] = true
	return true
}

func (r *Room) CheckTrigger() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.IsTriggered {
		return true // すでにトリガー済み
	}

	total := len(r.Participants)
	votes := len(r.VotedUsers)

	if total == 0 {
		return false
	}

	threshold := int(math.Ceil(float64(total) / 2.0))
	if votes >= threshold && votes > 0 {
		r.IsTriggered = true
		return true
	}
	return false
}

func (r *Room) GetState() (int, int, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.Participants), len(r.VotedUsers), r.IsTriggered
}

func (r *Room) GenerateGaugeHTML() string {
	total, votes, triggered := r.GetState()
	
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
