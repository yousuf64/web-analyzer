package analyzer

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// fetchContent fetches HTML content from a URL
func (s *Analyzer) fetchContent(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	start := time.Now()
	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	s.metrics.RecordHTTPClientRequest(resp.StatusCode, time.Since(start).Seconds(), req.Method, "content_fetch")

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("failed to fetch content: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	return string(body), nil
}
