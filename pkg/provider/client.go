package provider

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"time"
)

type Client struct {
	httpClient *http.Client
}

func NewClient(timeout time.Duration) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: timeout},
	}
}

// Call executes the upstream request and returns the raw *http.Response.
// The caller is responsible for closing the response body (each translator
// reads and closes it internally via TranslateResponse).
func (c *Client) Call(ctx context.Context, baseURL, path, apiKey string, reqBody []byte, headers map[string]string) (*http.Response, error) {
	url := baseURL + path

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("upstream call: %w", err)
	}
	return resp, nil
}
