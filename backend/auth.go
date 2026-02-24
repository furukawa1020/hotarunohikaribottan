package main

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
)

// ZoomAuthContext holds the decoded JWT payload from Zoom
type ZoomAuthContext struct {
	UID string `json:"uid"` // Unique user ID
	Mid string `json:"mid"` // Meeting ID
}

func getZoomClientSecret() string {
	// In production, this MUST be set
	secret := os.Getenv("ZOOM_CLIENT_SECRET")
	if secret == "" {
		log.Println("WARNING: ZOOM_CLIENT_SECRET is not set. Using dummy secret for development.")
		return "dummy_secret_for_local_dev"
	}
	return secret
}

// decodeBase64URL decodes base64url strings with or without padding
func decodeBase64URL(s string) ([]byte, error) {
	// Add padding if missing
	if m := len(s) % 4; m != 0 {
		s += strings.Repeat("=", 4-m)
	}
	return base64.URLEncoding.DecodeString(s)
}

// VerifyZoomContext decrypts the x-zoom-app-context header (AES-256-GCM) and returns the extracted Context
func VerifyZoomContext(appContext string) (*ZoomAuthContext, error) {
	if appContext == "" {
		return nil, fmt.Errorf("missing x-zoom-app-context header")
	}

	secret := getZoomClientSecret()

	b, err := decodeBase64URL(appContext)
	if err != nil {
		return nil, fmt.Errorf("base64 decode error: %w", err)
	}

	// Format: IV(16) + AAD(24) + CipherText(N) + AuthTag(16)
	// Min length 16+24+1+16 = 57
	if len(b) < 57 {
		return nil, fmt.Errorf("context payload too short")
	}

	iv := b[:16]
	aad := b[16:40]
	cipherText := b[40 : len(b)-16]
	authTag := b[len(b)-16:]

	// AES-256-GCM using SHA-256 of client_secret as the key
	hash := sha256.Sum256([]byte(secret))

	block, err := aes.NewCipher(hash[:])
	if err != nil {
		return nil, err
	}

	// Zoom uses a 16-byte IV for GCM instead of the standard 12
	aesgcm, err := cipher.NewGCMWithNonceSize(block, 16)
	if err != nil {
		return nil, err
	}

	// Go cipher.Open expects ciphertext and authTag to be concatenated
	cTextWithTag := append(cipherText, authTag...)

	plainText, err := aesgcm.Open(nil, iv, cTextWithTag, aad)
	if err != nil {
		return nil, fmt.Errorf("decrypt failed: %w", err)
	}

	// Parse JSON payload
	var payload map[string]interface{}
	if err := json.Unmarshal(plainText, &payload); err != nil {
		return nil, fmt.Errorf("json parse failed: %w", err)
	}

	var ctx ZoomAuthContext
	if uid, ok := payload["uid"].(string); ok {
		ctx.UID = uid
	}
	if mid, ok := payload["mid"].(string); ok {
		ctx.Mid = mid
	}

	if ctx.Mid == "" || ctx.UID == "" {
		return nil, fmt.Errorf("missing mid or uid in context payload")
	}

	return &ctx, nil
}

// AuthMiddleware extracts Zoom context from HTTP requests/WebSockets
func AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		appContext := r.Header.Get("x-zoom-app-context")
		if appContext == "" {
			appContext = r.URL.Query().Get("zoom_context")
		}

		// Skip verification if DEV_BYPASS=true (for pure local browser testing without Zoom)
		if os.Getenv("DEV_BYPASS") == "true" {
			ctx := context.WithValue(r.Context(), "zoomCtx", &ZoomAuthContext{
				Mid: r.URL.Query().Get("roomId"),
				UID: r.URL.Query().Get("pid"),
			})
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		zCtx, err := VerifyZoomContext(appContext)
		if err != nil {
			log.Printf("Authentication failed: %v", err)
			http.Error(w, "Unauthorized: Invalid Zoom Context", http.StatusUnauthorized)
			return
		}

		// Attach context to request
		ctx := context.WithValue(r.Context(), "zoomCtx", zCtx)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}
