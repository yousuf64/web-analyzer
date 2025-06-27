package main

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"shared/messagebus"
	"shared/types"

	"github.com/oklog/ulid/v2"
	"github.com/yousuf64/shift"
)

var validHostnameRegex = regexp.MustCompile(`^([a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?\.)*[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?$`)

// validateURL checks if the URL is valid and secure, and returns the normalized URL
func validateURL(rawURL string) (string, error) {
	if rawURL == "" {
		return "", errors.New("url is required")
	}

	// Remove leading/trailing whitespace
	rawURL = strings.TrimSpace(rawURL)

	// Check for maximum URL length (reasonable limit)
	if len(rawURL) > 2048 {
		return "", errors.New("url too long (max 2048 characters)")
	}

	// Add https:// if no scheme is present
	if !strings.Contains(rawURL, "://") {
		rawURL = "https://" + rawURL
	}

	// Parse the URL
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid url format: %w", err)
	}

	// Validate scheme - only allow http and https
	scheme := strings.ToLower(parsedURL.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", fmt.Errorf("unsupported scheme '%s': only http and https are allowed", parsedURL.Scheme)
	}

	// Validate hostname
	if parsedURL.Host == "" {
		return "", errors.New("hostname is required")
	}

	// Extract hostname (without port)
	hostname := parsedURL.Hostname()
	if hostname == "" {
		return "", errors.New("invalid hostname")
	}

	// Validate hostname format
	if err := validateHostname(hostname); err != nil {
		return "", fmt.Errorf("invalid hostname: %w", err)
	}

	// Check for path traversal patterns
	if strings.Contains(parsedURL.Path, "..") {
		return "", errors.New("path traversal patterns are not allowed")
	}

	return parsedURL.String(), nil
}

// validateHostname validates the hostname according to RFC standards and security considerations
func validateHostname(hostname string) error {
	// Check for localhost and loopback addresses
	if isLocalhost(hostname) {
		return errors.New("localhost and loopback addresses are not allowed")
	}

	// Check for private IP addresses
	if isPrivateIP(hostname) {
		return errors.New("private IP addresses are not allowed")
	}

	// Validate hostname format using regex
	if !validHostnameRegex.MatchString(hostname) {
		// If it's not a valid hostname, check if it's a valid IP
		if net.ParseIP(hostname) == nil {
			return errors.New("invalid hostname or IP address format")
		}
	}

	// Additional length check for hostname
	if len(hostname) > 253 {
		return errors.New("hostname too long (max 253 characters)")
	}

	return nil
}

// isLocalhost checks if the hostname is localhost or loopback
func isLocalhost(hostname string) bool {
	localhost := []string{
		"localhost",
		"127.0.0.1",
		"::1",
		"0.0.0.0",
	}

	hostname = strings.ToLower(hostname)
	for _, local := range localhost {
		if hostname == local {
			return true
		}
	}

	// Check for localhost variations
	if strings.HasSuffix(hostname, ".localhost") {
		return true
	}

	return false
}

// isPrivateIP checks if the hostname is a private IP address
func isPrivateIP(hostname string) bool {
	ip := net.ParseIP(hostname)
	if ip == nil {
		return false
	}

	// Check for private IP ranges
	privateRanges := []string{
		"10.0.0.0/8",     // RFC1918
		"172.16.0.0/12",  // RFC1918
		"192.168.0.0/16", // RFC1918
		"169.254.0.0/16", // RFC3927 (link-local)
		"fc00::/7",       // RFC4193 (IPv6 private)
		"fe80::/10",      // RFC4291 (IPv6 link-local)
	}

	for _, cidr := range privateRanges {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if network.Contains(ip) {
			return true
		}
	}

	return false
}

func handleAnalyze(w http.ResponseWriter, r *http.Request, route shift.Route) (err error) {
	ctx := r.Context()

	jobCreationStart := time.Now()
	defer func() {
		mc.RecordJobCreation(err == nil, time.Since(jobCreationStart))
	}()

	var req types.AnalyzeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return errors.Join(err, errors.New("failed to decode request"))
	}

	// Validate and normalize the URL
	validatedURL, err := validateURL(req.Url)
	if err != nil {
		return fmt.Errorf("url validation failed: %w", err)
	}
	req.Url = validatedURL

	jobId := generateId()
	logger.Info("Creating new analysis job",
		slog.String("jobId", jobId),
		slog.String("url", req.Url))

	job := &types.Job{
		ID:        jobId,
		URL:       req.Url,
		Status:    types.JobStatusPending,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	if err := jobRepo.CreateJob(ctx, job); err != nil {
		return errors.Join(err, errors.New("failed to create job"))
	}

	defaultTasks := getDefaultTasks(jobId)
	if err := taskRepo.CreateTasks(ctx, defaultTasks...); err != nil {
		return errors.Join(err, errors.New("failed to create tasks"))
	}

	if err := mb.PublishAnalyzeMessage(ctx, messagebus.AnalyzeMessage{
		Type:  messagebus.AnalyzeMessageType,
		JobId: jobId,
	}); err != nil {
		return errors.Join(err, errors.New("failed to publish analyze message"))
	}

	logger.Info("Analysis request published",
		slog.String("jobId", jobId),
		slog.String("url", req.Url))

	w.WriteHeader(http.StatusAccepted)
	return json.NewEncoder(w).Encode(types.AnalyzeResponse{
		Job: *job,
	})

}

func handleGetJobs(w http.ResponseWriter, r *http.Request, route shift.Route) (err error) {
	jobs, err := jobRepo.GetAllJobs(r.Context())
	if err != nil {
		return errors.Join(err, errors.New("failed to get jobs"))
	}

	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(jobs)
}

func handleGetTasksByJobId(w http.ResponseWriter, r *http.Request, route shift.Route) (err error) {
	jobId := route.Params.Get("job_id")
	if jobId == "" {
		return errors.New("job_id is required")
	}

	tasks, err := taskRepo.GetTasksByJobId(r.Context(), jobId)
	if err != nil {
		return errors.Join(err, errors.New("failed to get tasks"))
	}

	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(tasks)
}

// entropyPool provides a pool of monotonic entropy sources for ULID generation
// This allows for better performance in concurrent scenarios by avoiding lock contention
var entropyPool = sync.Pool{
	New: func() any {
		return ulid.Monotonic(rand.Reader, 0)
	},
}

func generateId() string {
	e := entropyPool.Get().(*ulid.MonotonicEntropy)

	ts := ulid.Timestamp(time.Now())
	id := ulid.MustNew(ts, e)

	entropyPool.Put(e)
	return id.String()
}

func getDefaultTasks(jobId string) []*types.Task {
	return []*types.Task{
		{
			JobID:  jobId,
			Type:   types.TaskTypeExtracting,
			Status: types.TaskStatusPending,
		},
		{
			JobID:  jobId,
			Type:   types.TaskTypeIdentifyingVersion,
			Status: types.TaskStatusPending,
		},
		{
			JobID:  jobId,
			Type:   types.TaskTypeAnalyzing,
			Status: types.TaskStatusPending,
		},
		{
			JobID:  jobId,
			Type:   types.TaskTypeVerifyingLinks,
			Status: types.TaskStatusPending,
		},
	}
}
