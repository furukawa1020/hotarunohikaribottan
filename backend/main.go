package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

// HTML rendering helper for HTMX
func generateGaugeHTML(fill float64, triggered bool) string {
	statusHtml := ""
	if fill >= 100 {
		statusHtml = `本日の営業は終了しました<br><span style="font-size: 0.6em">速やかにご退出ください</span>`
	} else if fill > 0 {
		statusHtml = `そろそろ… <span class='anonym-info'>(匿名)</span>`
	} else {
		statusHtml = `待機中 <span class='anonym-info'>(匿名)</span>`
	}

	triggerScript := ""
	if triggered {
		triggerScript = `<script>if(typeof audio !== 'undefined' && audio.paused) audio.play();</script>`
	}

	return fmt.Sprintf(`
<div id="gauge-container">
	<div class="gauge">
		<div class="gauge-fill" style="width: %.1f%%;"></div>
	</div>
	<p class="status-text">%s</p>
	%s
</div>`, fill, statusHtml, triggerScript)
}

func handleGetState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()
	zCtx, ok := ctx.Value("zoomCtx").(*ZoomAuthContext)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Calculate and return current state
	addParticipant(ctx, zCtx.Mid, zCtx.UID) // ensure active
	participants := getParticipantCount(ctx, zCtx.Mid)
	votes := getVoteCount(ctx, zCtx.Mid)

	fill := 0.0
	if participants > 0 {
		fill = (float64(votes) / float64(participants)) * 100
	}
	if fill > 100 {
		fill = 100
	}

	triggered := false
	if fill >= 100 {
		if !checkTrigger(ctx, zCtx.Mid) {
			setTrigger(ctx, zCtx.Mid)
		}
		triggered = true
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(generateGaugeHTML(fill, triggered)))
}

func handleVote(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()
	zCtx, ok := ctx.Value("zoomCtx").(*ZoomAuthContext)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	addVote(ctx, zCtx.Mid, zCtx.UID)

	// Just fetch and return updated state immediately
	handleGetState(w, r)
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

	fs := http.FileServer(http.Dir("../frontend"))
	mux := http.NewServeMux()

	// Intercept requests to inject the Zoom App Context header into index.html
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" || r.URL.Path == "/index.html" {
			htmlBytes, err := os.ReadFile("../frontend/index.html")
			if err != nil {
				http.Error(w, "Failed to load index.html", http.StatusInternalServerError)
				return
			}

			htmlStr := string(htmlBytes)
			ctxHeader := r.Header.Get("x-zoom-app-context")

			// Inject the context directly into a meta tag
			metaTag := fmt.Sprintf(`<meta name="zoom-app-context" content="%s">`, ctxHeader)
			htmlStr = strings.Replace(htmlStr, "</head>", metaTag+"\n</head>", 1)

			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write([]byte(htmlStr))
			return
		}

		fs.ServeHTTP(w, r)
	})

	// Start HTTP Endpoints (No WebSockets)
	mux.HandleFunc("/api/state", AuthMiddleware(handleGetState))
	mux.HandleFunc("/api/vote", AuthMiddleware(handleVote))
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
