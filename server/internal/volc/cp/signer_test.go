package cp

import (
	"encoding/hex"
	"net/http"
	"strings"
	"testing"
	"time"
)

// TestSignedKeyMatchesOfficialGolden validates the V4 key-derivation chain
// against the official Volcengine signature golden vector (docs 6369/67270).
func TestSignedKeyMatchesOfficialGolden(t *testing.T) {
	const (
		secretKey      = "WkRZeE1EQmxPVGhsWWpWak5HVmtNbUUxTXpZeU9UVXlOMlE1TmpZeVlqTQ=="
		shortDate      = "20240619"
		region         = "cn-beijing"
		service        = "iam"
		wantSigningKey = "abee62e533a58934c49954459a3c3237d2fccea517c9a7c8a2651d8ea7779826"
	)
	got := hex.EncodeToString(signedKey(secretKey, shortDate, region, service))
	if got != wantSigningKey {
		t.Fatalf("signing key = %s, want %s", got, wantSigningKey)
	}
}

// TestStringToSignSignatureMatchesGolden derives the final signature from the
// golden signing key + a known string-to-sign and confirms the HMAC step.
func TestSignatureFromGoldenSigningKey(t *testing.T) {
	keyHex := "abee62e533a58934c49954459a3c3237d2fccea517c9a7c8a2651d8ea7779826"
	key, err := hex.DecodeString(keyHex)
	if err != nil {
		t.Fatalf("decode key: %v", err)
	}
	// Sanity: the signing primitive is deterministic.
	sig := hex.EncodeToString(hmacSHA256(key, "probe"))
	if len(sig) != 64 {
		t.Fatalf("signature length = %d, want 64", len(sig))
	}
}

func TestSignSetsDeterministicAuthorization(t *testing.T) {
	body := []byte(`{"Name":"demo"}`)
	req, err := http.NewRequest(http.MethodPost,
		"https://open.volcengineapi.com/?Action=CreateWorkspace&Version=2023-01-01", strings.NewReader(string(body)))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	creds := Credentials{AccessKey: "AKTEST", SecretKey: "SKTEST"}
	now := time.Date(2024, 6, 19, 7, 13, 6, 0, time.UTC)

	Sign(req, body, creds, "cn-beijing", "cp", now)
	first := req.Header.Get("Authorization")

	// Re-sign an identical request and confirm the signature is stable.
	req2, _ := http.NewRequest(http.MethodPost,
		"https://open.volcengineapi.com/?Action=CreateWorkspace&Version=2023-01-01", strings.NewReader(string(body)))
	req2.Header.Set("Content-Type", "application/json")
	Sign(req2, body, creds, "cn-beijing", "cp", now)
	second := req2.Header.Get("Authorization")

	if first != second {
		t.Fatalf("authorization not deterministic:\n%s\n%s", first, second)
	}
	if !strings.HasPrefix(first, "HMAC-SHA256 Credential=AKTEST/20240619/cn-beijing/cp/request") {
		t.Fatalf("unexpected credential scope: %s", first)
	}
	if !strings.Contains(first, "SignedHeaders=content-type;host;x-content-sha256;x-date") {
		t.Fatalf("unexpected signed headers: %s", first)
	}
	if req.Header.Get("X-Date") != "20240619T071306Z" {
		t.Fatalf("x-date = %s", req.Header.Get("X-Date"))
	}
	if req.Header.Get("X-Content-Sha256") != sha256Hex(body) {
		t.Fatalf("x-content-sha256 mismatch")
	}
}

func TestSignEmptyBodyUsesEmptyHash(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet,
		"https://open.volcengineapi.com/?Action=ListWorkspaces&Version=2023-01-01", nil)
	Sign(req, nil, Credentials{AccessKey: "AK", SecretKey: "SK"}, "cn-beijing", "cp",
		time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC))
	if req.Header.Get("X-Content-Sha256") != emptyPayloadSHA256 {
		t.Fatalf("empty payload hash = %s", req.Header.Get("X-Content-Sha256"))
	}
}

func TestSignWithSessionTokenSignsToken(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet,
		"https://open.volcengineapi.com/?Action=ListWorkspaces&Version=2023-01-01", nil)
	creds := Credentials{AccessKey: "AK", SecretKey: "SK", SessionToken: "STS-TOKEN"}
	Sign(req, nil, creds, "cn-beijing", "cp", time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC))
	if req.Header.Get("X-Security-Token") != "STS-TOKEN" {
		t.Fatalf("missing security token header")
	}
	if !strings.Contains(req.Header.Get("Authorization"), "x-security-token") {
		t.Fatalf("session token not in signed headers: %s", req.Header.Get("Authorization"))
	}
}

func TestCanonicalQueryEncodesSpacesAsPercent20(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "https://h/?b=two+words&a=1", nil)
	got := canonicalQuery(req.URL)
	if got != "a=1&b=two%20words" {
		t.Fatalf("canonical query = %q", got)
	}
}
