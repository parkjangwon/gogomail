package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

const maxRequestBytes = 4096

type SignRequest struct {
	Algorithm       string `json:"algorithm"`
	KeyID           string `json:"key_id"`
	SignedDigestHex string `json:"signed_digest_hex"`
}

type SignResponse struct {
	Algorithm       string `json:"algorithm"`
	KeyID           string `json:"key_id"`
	SignedDigestHex string `json:"signed_digest_hex"`
	SignatureHex    string `json:"signature_hex"`
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	keyID := os.Getenv("SIGNER_KEY_ID")
	if keyID == "" {
		log.Fatal("SIGNER_KEY_ID is required")
	}

	privKeyBase64 := os.Getenv("SIGNER_PRIVATE_KEY")
	if privKeyBase64 == "" {
		log.Fatal("SIGNER_PRIVATE_KEY is required (base64 encoded ed25519 private key)")
	}

	privKeyBytes, err := base64.StdEncoding.DecodeString(privKeyBase64)
	if err != nil {
		log.Fatalf("invalid SIGNER_PRIVATE_KEY: %v", err)
	}
	if len(privKeyBytes) != ed25519.PrivateKeySize {
		log.Fatalf("SIGNER_PRIVATE_KEY must be exactly %d bytes", ed25519.PrivateKeySize)
	}
	privKey := ed25519.PrivateKey(privKeyBytes)

	token := os.Getenv("SIGNER_AUTH_TOKEN")

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/sign", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}

		if token != "" {
			authHeader := r.Header.Get("Authorization")
			if authHeader != "Bearer "+token {
				http.Error(w, `{"error":"Unauthorized"}`, http.StatusUnauthorized)
				return
			}
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBytes)
		defer r.Body.Close()

		var req SignRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"Bad request"}`, http.StatusBadRequest)
			return
		}

		if req.Algorithm != "ed25519" {
			http.Error(w, `{"error":"Unsupported algorithm"}`, http.StatusBadRequest)
			return
		}
		if req.KeyID != keyID {
			http.Error(w, `{"error":"Unknown key_id"}`, http.StatusBadRequest)
			return
		}
		digestHex := strings.ToLower(strings.TrimSpace(req.SignedDigestHex))
		if len(digestHex) != 64 {
			http.Error(w, `{"error":"Invalid digest hex"}`, http.StatusBadRequest)
			return
		}
		for _, c := range digestHex {
			if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
				http.Error(w, `{"error":"Invalid digest hex format"}`, http.StatusBadRequest)
				return
			}
		}

		sigBytes := ed25519.Sign(privKey, []byte(digestHex))

		resp := SignResponse{
			Algorithm:       "ed25519",
			KeyID:           keyID,
			SignedDigestHex: digestHex,
			SignatureHex:    hex.EncodeToString(sigBytes),
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	log.Printf("Starting remote signer on port %s for key %s", port, keyID)
	log.Fatal(srv.ListenAndServe())
}
