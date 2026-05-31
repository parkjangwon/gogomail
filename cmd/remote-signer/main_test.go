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
	"strings"
	"testing"
	"time"
)

func TestSigner(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	keyID := "test-key-1"
	token := "secret-token"
	digestHex := strings.Repeat("a", 64)

	ts := httptest.NewServer(newSignHandler(signerConfig{
		KeyID:      keyID,
		PrivateKey: priv,
		Token:      token,
	}))
	defer ts.Close()

	reqBody := `{"algorithm":"ed25519","key_id":"test-key-1","signed_digest_hex":"` + digestHex + `"}`
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/v1/sign", bytes.NewReader([]byte(reqBody)))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var signed SignResponse
	if err := json.NewDecoder(resp.Body).Decode(&signed); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	sig, err := hex.DecodeString(signed.SignatureHex)
	if err != nil {
		t.Fatalf("decode signature: %v", err)
	}
	if signed.KeyID != keyID || signed.SignedDigestHex != digestHex || signed.Algorithm != "ed25519" {
		t.Fatalf("signed response = %+v", signed)
	}
	if !ed25519.Verify(pub, []byte(digestHex), sig) {
		t.Fatal("signature does not verify")
	}
}

func TestLoadSignerConfigValidatesRequiredEnv(t *testing.T) {
	t.Parallel()

	_, err := loadSignerConfig(func(string) string { return "" })
	if err == nil || !strings.Contains(err.Error(), "SIGNER_KEY_ID is required") {
		t.Fatalf("loadSignerConfig err = %v, want missing key id", err)
	}
}

func TestLoadSignerConfigParsesEnvironment(t *testing.T) {
	t.Parallel()

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	values := map[string]string{
		"PORT":               "9090",
		"SIGNER_KEY_ID":      "key-1",
		"SIGNER_PRIVATE_KEY": base64.StdEncoding.EncodeToString(priv),
		"SIGNER_AUTH_TOKEN":  "token-1",
	}
	cfg, err := loadSignerConfig(func(key string) string { return values[key] })
	if err != nil {
		t.Fatalf("loadSignerConfig returned error: %v", err)
	}
	if cfg.Port != "9090" || cfg.KeyID != "key-1" || cfg.Token != "token-1" || !bytes.Equal(cfg.PrivateKey, priv) {
		t.Fatalf("config = %+v", cfg)
	}
}

func TestNewSignerServerSetsOperationalTimeouts(t *testing.T) {
	t.Parallel()

	srv := newSignerServer(":0", http.NewServeMux())
	if srv.ReadHeaderTimeout != 5*time.Second || srv.ReadTimeout != 10*time.Second || srv.WriteTimeout != 10*time.Second || srv.IdleTimeout != 60*time.Second {
		t.Fatalf("server timeouts = readHeader %s read %s write %s idle %s", srv.ReadHeaderTimeout, srv.ReadTimeout, srv.WriteTimeout, srv.IdleTimeout)
	}
	if srv.MaxHeaderBytes != 8<<10 {
		t.Fatalf("MaxHeaderBytes = %d, want 8192", srv.MaxHeaderBytes)
	}
}
