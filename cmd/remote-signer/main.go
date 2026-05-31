package main

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

const maxRequestBytes = 4096
const signerShutdownTimeout = 10 * time.Second

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

type signerConfig struct {
	Port       string
	KeyID      string
	PrivateKey ed25519.PrivateKey
	Token      string
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Getenv, logger); err != nil {
		logger.Error("remote signer stopped", "error", err)
		os.Exit(1)
	}
}

func loadSignerConfig(getenv func(string) string) (signerConfig, error) {
	port := strings.TrimSpace(getenv("PORT"))
	if port == "" {
		port = "8080"
	}
	keyID := strings.TrimSpace(getenv("SIGNER_KEY_ID"))
	if keyID == "" {
		return signerConfig{}, fmt.Errorf("SIGNER_KEY_ID is required")
	}
	privKeyBase64 := strings.TrimSpace(getenv("SIGNER_PRIVATE_KEY"))
	if privKeyBase64 == "" {
		return signerConfig{}, fmt.Errorf("SIGNER_PRIVATE_KEY is required (base64 encoded ed25519 private key)")
	}
	privKeyBytes, err := base64.StdEncoding.DecodeString(privKeyBase64)
	if err != nil {
		return signerConfig{}, fmt.Errorf("invalid SIGNER_PRIVATE_KEY: %w", err)
	}
	if len(privKeyBytes) != ed25519.PrivateKeySize {
		return signerConfig{}, fmt.Errorf("SIGNER_PRIVATE_KEY must be exactly %d bytes", ed25519.PrivateKeySize)
	}
	return signerConfig{
		Port:       port,
		KeyID:      keyID,
		PrivateKey: ed25519.PrivateKey(privKeyBytes),
		Token:      getenv("SIGNER_AUTH_TOKEN"),
	}, nil
}

func newSignHandler(cfg signerConfig) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/sign", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}

		if cfg.Token != "" {
			authHeader := r.Header.Get("Authorization")
			if authHeader != "Bearer "+cfg.Token {
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
		if req.KeyID != cfg.KeyID {
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

		sigBytes := ed25519.Sign(cfg.PrivateKey, []byte(digestHex))

		resp := SignResponse{
			Algorithm:       "ed25519",
			KeyID:           cfg.KeyID,
			SignedDigestHex: digestHex,
			SignatureHex:    hex.EncodeToString(sigBytes),
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			slog.WarnContext(r.Context(), "remote signer response encode failed", "error", err)
		}
	})
	return mux
}

func newSignerServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    8 << 10,
	}
}

func run(ctx context.Context, getenv func(string) string, logger *slog.Logger) error {
	if logger == nil {
		logger = slog.Default()
	}
	cfg, err := loadSignerConfig(getenv)
	if err != nil {
		return err
	}
	srv := newSignerServer(":"+cfg.Port, newSignHandler(cfg))
	listener, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		return fmt.Errorf("listen remote signer: %w", err)
	}
	errCh := make(chan error, 1)
	go func() {
		logger.Info("remote signer listening", "addr", listener.Addr().String(), "key_id", cfg.KeyID)
		errCh <- srv.Serve(listener)
	}()

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		logger.Info("remote signer shutting down", "reason", ctx.Err())
		shutdownCtx, cancel := context.WithTimeout(context.Background(), signerShutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown remote signer: %w", err)
		}
		if err := <-errCh; err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}
}
