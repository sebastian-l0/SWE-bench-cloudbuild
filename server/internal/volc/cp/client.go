package cp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// APIError is the unified CP error model surfaced to callers.
type APIError struct {
	HTTPStatus int
	Code       string
	Message    string
	RequestID  string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("cp: api error status=%d code=%s request_id=%s: %s",
		e.HTTPStatus, e.Code, e.RequestID, e.Message)
}

// responseMetadata mirrors the common Volcengine ResponseMetadata envelope.
type responseMetadata struct {
	RequestID string `json:"RequestId"`
	Action    string `json:"Action"`
	Version   string `json:"Version"`
	Service   string `json:"Service"`
	Region    string `json:"Region"`
	Error     *struct {
		Code    string `json:"Code"`
		Message string `json:"Message"`
	} `json:"Error,omitempty"`
}

type envelope struct {
	ResponseMetadata responseMetadata `json:"ResponseMetadata"`
	Result           json.RawMessage  `json:"Result"`
}

// HTTPClient is the real CP API client. It signs requests with V4-HMAC-SHA256
// and decodes the common response envelope into Result or APIError.
type HTTPClient struct {
	target Target
	creds  Credentials
	http   *http.Client
	now    func() time.Time
}

// NewHTTPClient builds a CP HTTP client for the resolved target.
func NewHTTPClient(target Target, creds Credentials) *HTTPClient {
	return &HTTPClient{
		target: target,
		creds:  creds,
		http:   &http.Client{Timeout: 30 * time.Second},
		now:    time.Now,
	}
}

// Call invokes a CP RPC action. The body is marshaled as JSON for POST actions;
// pass nil for parameterless GET actions. The decoded Result is unmarshaled into
// out when non-nil.
func (c *HTTPClient) Call(ctx context.Context, method, action string, body any, out any) error {
	query := url.Values{}
	query.Set("Action", action)
	query.Set("Version", c.target.Version)
	endpoint := url.URL{
		Scheme:   "https",
		Host:     c.target.Host,
		Path:     "/",
		RawQuery: query.Encode(),
	}

	var payload []byte
	if body != nil {
		marshaled, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("cp: marshal request: %w", err)
		}
		payload = marshaled
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint.String(), bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("cp: build request: %w", err)
	}
	req.Host = c.target.Host
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	Sign(req, payload, c.creds, c.target.Region, c.target.Service, c.now())

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("cp: do request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("cp: read response: %w", err)
	}

	var env envelope
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &env); err != nil {
			return fmt.Errorf("cp: decode response (status %d): %w", resp.StatusCode, err)
		}
	}

	if env.ResponseMetadata.Error != nil || resp.StatusCode >= http.StatusBadRequest {
		apiErr := &APIError{
			HTTPStatus: resp.StatusCode,
			RequestID:  env.ResponseMetadata.RequestID,
		}
		if env.ResponseMetadata.Error != nil {
			apiErr.Code = env.ResponseMetadata.Error.Code
			apiErr.Message = env.ResponseMetadata.Error.Message
		} else {
			apiErr.Code = "HTTPError"
			apiErr.Message = string(raw)
		}
		return apiErr
	}

	if out != nil && len(env.Result) > 0 {
		if err := json.Unmarshal(env.Result, out); err != nil {
			return fmt.Errorf("cp: decode result: %w", err)
		}
	}
	return nil
}
