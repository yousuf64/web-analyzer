package analyzer

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"shared/messagebus"
	"shared/models"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

// verifyLinks verifies all collected links concurrently
func (s *Analyzer) verifyLinks(ctx context.Context, jobID string, result *AnalysisResult) {
	start := time.Now()
	s.updateTaskStatus(ctx, jobID, models.TaskTypeVerifyingLinks, models.TaskStatusRunning)

	defer func() {
		s.updateTaskStatus(ctx, jobID, models.TaskTypeVerifyingLinks, models.TaskStatusCompleted)
		if s.metrics != nil {
			s.metrics.RecordAnalysisTask(string(models.TaskTypeVerifyingLinks), true, time.Since(start).Seconds())
		}
	}()

	count := len(result.links)
	if count == 0 {
		return
	}

	s.log.Info("Starting link verification", "linkCount", count)

	// Track concurrent link verifications
	if s.metrics != nil {
		s.metrics.SetConcurrentLinkVerifications(count)
		defer s.metrics.SetConcurrentLinkVerifications(0)
	}

	maxConcurrent := 10
	if s.cfg != nil {
		maxConcurrent = s.cfg.HTTP.MaxConcurrent
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, maxConcurrent)

	for i, link := range result.links {
		key := strconv.Itoa(i + 1)
		s.publishSubTaskAdd(ctx, jobID, models.TaskTypeVerifyingLinks, key, link)

		s.log.Debug("Added subtask for link verification", "key", key, "url", link)

		wg.Add(1)
		go func(ctx context.Context, link, key string) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() {
				<-sem
			}()

			s.publishSubTaskUpdate(ctx, jobID, models.TaskTypeVerifyingLinks, key, models.SubTask{
				Type:   models.SubTaskTypeValidatingLink,
				Status: models.TaskStatusRunning,
				URL:    link,
			})

			start := time.Now()
			status, desc := s.verifyLink(ctx, link)
			d := time.Since(start).Seconds()

			s.publishSubTaskUpdate(ctx, jobID, models.TaskTypeVerifyingLinks, key, models.SubTask{
				Type:        models.SubTaskTypeValidatingLink,
				Status:      status,
				URL:         link,
				Description: desc,
			})

			if status == models.TaskStatusCompleted {
				atomic.AddInt32(&result.accessibleLinks, 1)
			} else {
				atomic.AddInt32(&result.inaccessibleLinks, 1)
			}

			if s.metrics != nil {
				s.metrics.RecordLinkVerification(status == models.TaskStatusCompleted, d)
			}

		}(ctx, link, key)
	}

	wg.Wait()
	s.log.Info("Completed link verification", "linkCount", count)
}

// verifyLink verifies a single link
func (s *Analyzer) verifyLink(ctx context.Context, link string) (models.TaskStatus, string) {
	u, err := url.Parse(link)
	if err != nil {
		msg := fmt.Sprintf("Invalid URL: %s", err.Error())
		s.log.Error("Error parsing URL", "url", link, "error", err)
		return models.TaskStatusFailed, msg
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		desc := fmt.Sprintf("Unsupported protocol: %s", u.Scheme)
		s.log.Debug("Skipping non-HTTP URL", "url", link, "scheme", u.Scheme)
		return models.TaskStatusSkipped, desc
	}

	// Start with HEAD request
	status, desc, retry := s.tryHEADRequest(ctx, link)

	// If HEAD failed with specific errors that suggest GET might work, retry with GET
	if retry {
		s.log.Debug("Retrying with GET request", "url", link, "reason", "HEAD request failed or not supported")
		status, desc = s.tryGETRequest(ctx, link)
	}

	return status, desc
}

// tryHEADRequest attempts to verify a link using HEAD request
func (s *Analyzer) tryHEADRequest(ctx context.Context, link string) (models.TaskStatus, string, bool) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, link, nil)
	if err != nil {
		msg := fmt.Sprintf("HEAD request creation failed: %s", err.Error())
		s.log.Error("Failed to create HEAD request", "url", link, "error", err)
		return models.TaskStatusFailed, msg, false
	}

	start := time.Now()
	resp, err := s.client.Do(req)
	if err != nil {
		msg := s.formatRequestError(err)
		s.log.Debug("HEAD request failed", "url", link, "error", err)
		if s.metrics != nil {
			s.metrics.RecordHTTPClientRequest(0, time.Since(start).Seconds(), http.MethodHead, "link_verification")
		}
		return models.TaskStatusFailed, msg, false
	}
	defer resp.Body.Close()

	if s.metrics != nil {
		s.metrics.RecordHTTPClientRequest(resp.StatusCode, time.Since(start).Seconds(), http.MethodHead, "link_verification")
	}

	// Check if we should retry with GET
	retry := s.shouldRetryWithGET(resp.StatusCode)

	if retry {
		return models.TaskStatusPending, "HEAD not supported, retrying with GET", true
	}

	// Process successful HEAD response
	desc := s.formatResponse(resp)

	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		s.log.Debug("Link verified with HEAD", "url", link, "statusCode", resp.StatusCode)
		return models.TaskStatusCompleted, desc, false
	}

	s.log.Debug("Link verification failed with HEAD", "url", link, "statusCode", resp.StatusCode)
	return models.TaskStatusFailed, desc, false
}

// tryGETRequest attempts to verify a link using GET request (fallback)
func (s *Analyzer) tryGETRequest(ctx context.Context, link string) (models.TaskStatus, string) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, link, nil)
	if err != nil {
		msg := fmt.Sprintf("GET request creation failed: %s", err.Error())
		s.log.Error("Failed to create GET request", "url", link, "error", err)
		return models.TaskStatusFailed, msg
	}

	start := time.Now()
	resp, err := s.client.Do(req)
	if err != nil {
		msg := s.formatRequestError(err)
		s.log.Error("GET request failed", "url", link, "error", err)
		if s.metrics != nil {
			s.metrics.RecordHTTPClientRequest(0, time.Since(start).Seconds(), http.MethodGet, "link_verification")
		}
		return models.TaskStatusFailed, msg
	}
	defer resp.Body.Close()

	if s.metrics != nil {
		s.metrics.RecordHTTPClientRequest(resp.StatusCode, time.Since(start).Seconds(), http.MethodGet, "link_verification")
	}

	desc := s.formatResponse(resp)

	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		s.log.Debug("Link verified with GET", "url", link, "statusCode", resp.StatusCode)
		return models.TaskStatusCompleted, desc
	}

	s.log.Debug("Link verification failed with GET", "url", link, "statusCode", resp.StatusCode)
	return models.TaskStatusFailed, desc
}

// shouldRetryWithGET determines if we should retry a failed HEAD request with GET
func (s *Analyzer) shouldRetryWithGET(statusCode int) bool {
	switch statusCode {
	case http.StatusMethodNotAllowed: // Server doesn't support HEAD
		return true
	case http.StatusNotImplemented: // Server doesn't implement HEAD
		return true
	case http.StatusBadRequest: // Account for servers that return 400 when they should return 405
		return true
	default:
		return false
	}
}

// formatRequestError formats HTTP request errors consistently
func (s *Analyzer) formatRequestError(err error) string {
	if urlErr, ok := err.(*url.Error); ok {
		if urlErr.Timeout() {
			return "Connection timeout"
		} else {
			return fmt.Sprintf("Connection error: %s", urlErr.Err.Error())
		}
	}
	return fmt.Sprintf("Request failed: %s", err.Error())
}

// formatResponse formats HTTP response information consistently
func (s *Analyzer) formatResponse(resp *http.Response) string {
	description := fmt.Sprintf("HTTP %d: %s", resp.StatusCode, http.StatusText(resp.StatusCode))

	// Check for redirects
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		if location, ok := resp.Header["Location"]; ok && len(location) > 0 {
			description = fmt.Sprintf("HTTP %d: Redirected to %s", resp.StatusCode, location[0])
		}
	}

	return description
}

// publishSubTaskAdd publishes a subtask add event
func (s *Analyzer) publishSubTaskAdd(ctx context.Context, jobID string, taskType models.TaskType, key, url string) {
	subTask := models.SubTask{
		Type:   models.SubTaskTypeValidatingLink,
		Status: models.TaskStatusPending,
		URL:    url,
	}

	if err := s.publisher.PublishSubTaskUpdate(ctx, messagebus.SubTaskUpdateMessage{
		Type:     messagebus.SubTaskUpdateMessageType,
		JobID:    jobID,
		TaskType: string(taskType),
		Key:      key,
		SubTask:  subTask,
	}); err != nil {
		s.log.Error("Failed to publish subtask add", "error", err)
	}
}

// publishSubTaskUpdate publishes a subtask update event
func (s *Analyzer) publishSubTaskUpdate(ctx context.Context, jobID string, taskType models.TaskType, key string, subtask models.SubTask) {
	if err := s.publisher.PublishSubTaskUpdate(ctx, messagebus.SubTaskUpdateMessage{
		Type:     messagebus.SubTaskUpdateMessageType,
		JobID:    jobID,
		TaskType: string(taskType),
		Key:      key,
		SubTask:  subtask,
	}); err != nil {
		s.log.Error("Failed to publish subtask update", "error", err)
	}
}
