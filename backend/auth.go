package main

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
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
	secret := strings.TrimSpace(os.Getenv("ZOOM_CLIENT_SECRET"))
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

	if len(b) < 1 {
		return nil, fmt.Errorf("context payload too short (no ivLength)")
	}

	offset := 0
	ivLength := int(b[offset])
	offset += 1

	if len(b) < offset+ivLength {
		return nil, fmt.Errorf("context payload too short (iv)")
	}
	iv := b[offset : offset+ivLength]
	offset += ivLength

	if len(b) < offset+2 {
		return nil, fmt.Errorf("context payload too short (aadLength)")
	}
	aadLength := int(binary.LittleEndian.Uint16(b[offset : offset+2]))
	offset += 2

	if len(b) < offset+aadLength {
		return nil, fmt.Errorf("context payload too short (aad)")
	}
	aad := b[offset : offset+aadLength]
	offset += aadLength

	if len(b) < offset+4 {
		return nil, fmt.Errorf("context payload too short (cipherTextLength)")
	}
	cipherTextLength := int(binary.LittleEndian.Uint32(b[offset : offset+4]))
	offset += 4

	if len(b) < offset+cipherTextLength {
		return nil, fmt.Errorf("context payload too short (cipherText)")
	}
	cipherText := b[offset : offset+cipherTextLength]
	offset += cipherTextLength

	// The remaining bytes are the auth tag (usually 16 bytes for GCM)
	authTag := b[offset:]

	// Zoom uses AES-256-GCM using SHA-256 of client_secret as the key
	hash := sha256.Sum256([]byte(secret))

	block, err := aes.NewCipher(hash[:])
	if err != nil {
		return nil, err
	}

	aesgcm, err := cipher.NewGCMWithNonceSize(block, len(iv))
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
		if strings.TrimSpace(os.Getenv("DEV_BYPASS")) == "true" {
			log.Println("[DEBUG] DEV_BYPASS is active. Bypassing Zoom Auth.")
			ctx := context.WithValue(r.Context(), "zoomCtx", &ZoomAuthContext{
				Mid: r.URL.Query().Get("roomId"),
				UID: r.URL.Query().Get("pid"),
			})
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		log.Printf("[DEBUG] Incoming request to %s from %s", r.URL.Path, r.RemoteAddr)
		if appContext == "" {
			log.Println("[DEBUG] Authentication failed: Missing x-zoom-app-context header and query param")
			http.Error(w, "Unauthorized: Context Missing", http.StatusUnauthorized)
			return
		}

		zCtx, err := VerifyZoomContext(appContext)
		if err != nil {
			log.Printf("[DEBUG] Authentication failed for context verification: %v (appContext: %s)", err, appContext)
			http.Error(w, "Unauthorized: Invalid Zoom Context", http.StatusUnauthorized)
			return
		}

		log.Printf("[DEBUG] Zoom Auth Successful. UID: %s, Mid: %s", zCtx.UID, zCtx.Mid)

		// Attach context to request
		ctx := context.WithValue(r.Context(), "zoomCtx", zCtx)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}
