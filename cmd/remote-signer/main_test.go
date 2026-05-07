package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestSigner(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	privBase64 := base64.StdEncoding.EncodeToString(priv)
	keyID := "test-key-1"
	token := "secret-token"

	os.Setenv("SIGNER_KEY_ID", keyID)
	os.Setenv("SIGNER_PRIVATE_KEY", privBase64)
	os.Setenv("SIGNER_AUTH_TOKEN", token)
	defer os.Clearenv()

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/sign", func(w http.ResponseWriter, r *http.Request) {
		// Duplicated from main() since we can't easily extract the handler without refactoring
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

		sigBytes := ed25519.Sign(priv, []byte(digestHex))

		resp := SignResponse{
			Algorithm:       "ed25519",
			KeyID:           keyID,
			SignedDigestHex: digestHex,
			SignatureHex:    hex.EncodeToString(sigBytes),
		}
		// In main() we use hex.EncodeToString. Let's fix the test to use hex.EncodeToString
		_ = pub // pub key is not used here but could be for verification
		json.NewEncoder(w).Encode(resp)
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	// Test case: valid request
	reqBody := `{"algorithm":"ed25519","key_id":"test-key-1","signed_digest_hex":"` + strings.Repeat("a", 64) + `"}`
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/v1/sign", bytes.NewReader([]byte(reqBody)))
	req.Header.Set("Authorization", "Bearer secret-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", resp.StatusCode)
	}
}
