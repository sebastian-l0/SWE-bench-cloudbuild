package cp

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// Credentials holds the Volcengine access key pair and optional STS token.
type Credentials struct {
	AccessKey    string
	SecretKey    string
	SessionToken string
}

// emptyPayloadSHA256 is the SHA256 of an empty body.
const emptyPayloadSHA256 = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

// signedKey derives the Volcengine V4 signing key. Unlike AWS SigV4 the raw
// secret key is used directly (no "AWS4" prefix) and the trailing constant is
// "request" rather than "aws4_request".
func signedKey(secretKey, shortDate, region, service string) []byte {
	kDate := hmacSHA256([]byte(secretKey), shortDate)
	kRegion := hmacSHA256(kDate, region)
	kService := hmacSHA256(kRegion, service)
	return hmacSHA256(kService, "request")
}

func hmacSHA256(key []byte, content string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(content))
	return mac.Sum(nil)
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// Sign signs req in place with Volcengine V4-HMAC-SHA256. The body must be the
// exact bytes that will be sent. now is the signing time (UTC). region and
// service form the credential scope together with the date.
func Sign(req *http.Request, body []byte, creds Credentials, region, service string, now time.Time) {
	now = now.UTC()
	date := now.Format("20060102T150405Z")
	shortDate := date[:8]

	payloadHash := emptyPayloadSHA256
	if len(body) > 0 {
		payloadHash = sha256Hex(body)
	}

	req.Header.Set("X-Date", date)
	req.Header.Set("X-Content-Sha256", payloadHash)
	if creds.SessionToken != "" {
		req.Header.Set("X-Security-Token", creds.SessionToken)
	}
	host := req.Host
	if host == "" {
		host = req.URL.Host
	}

	// Build the set of headers to sign.
	signed := map[string]string{
		"host":             host,
		"x-date":           date,
		"x-content-sha256": payloadHash,
	}
	if ct := req.Header.Get("Content-Type"); ct != "" {
		signed["content-type"] = ct
	}
	if creds.SessionToken != "" {
		signed["x-security-token"] = creds.SessionToken
	}

	names := make([]string, 0, len(signed))
	for name := range signed {
		names = append(names, name)
	}
	sort.Strings(names)

	var canonicalHeaders strings.Builder
	for _, name := range names {
		canonicalHeaders.WriteString(name)
		canonicalHeaders.WriteByte(':')
		canonicalHeaders.WriteString(strings.TrimSpace(signed[name]))
		canonicalHeaders.WriteByte('\n')
	}
	signedHeaders := strings.Join(names, ";")

	canonicalRequest := strings.Join([]string{
		req.Method,
		canonicalURI(req.URL),
		canonicalQuery(req.URL),
		canonicalHeaders.String(),
		signedHeaders,
		payloadHash,
	}, "\n")

	credentialScope := shortDate + "/" + region + "/" + service + "/request"
	stringToSign := strings.Join([]string{
		"HMAC-SHA256",
		date,
		credentialScope,
		sha256Hex([]byte(canonicalRequest)),
	}, "\n")

	key := signedKey(creds.SecretKey, shortDate, region, service)
	signature := hex.EncodeToString(hmacSHA256(key, stringToSign))

	authorization := "HMAC-SHA256 Credential=" + creds.AccessKey + "/" + credentialScope +
		", SignedHeaders=" + signedHeaders +
		", Signature=" + signature
	req.Header.Set("Authorization", authorization)
}

func canonicalURI(u *url.URL) string {
	if u.Path == "" {
		return "/"
	}
	return u.EscapedPath()
}

// canonicalQuery sorts query parameters by key and encodes spaces as %20.
func canonicalQuery(u *url.URL) string {
	return strings.ReplaceAll(u.Query().Encode(), "+", "%20")
}
