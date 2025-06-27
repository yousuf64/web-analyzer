package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"shared/metrics"
	"shared/types"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/net/html"
)

const (
	maxConc = 10
)

type TaskStatusUpdateFunc func(taskType types.TaskType, status types.TaskStatus)
type AddSubTaskFunc func(taskType types.TaskType, key, url string)
type SubTaskUpdateFunc func(taskType types.TaskType, key string, subtask types.SubTask)

type Analyzer struct {
	htmlVersion       string
	title             string
	headings          map[string]int
	links             []string
	internalLinks     int32
	externalLinks     int32
	accessibleLinks   int32
	inaccessibleLinks int32
	hasLoginForm      bool
	baseUrl           string

	hc *http.Client
	mc metrics.AnalyzerMetricsInterface

	TaskStatusUpdateFunc TaskStatusUpdateFunc
	AddSubTaskFunc       AddSubTaskFunc
	SubTaskUpdateFunc    SubTaskUpdateFunc
}

func NewAnalyzer(hc *http.Client, m metrics.AnalyzerMetricsInterface) *Analyzer {
	if m == nil {
		m = metrics.NewNoopAnalyzerMetrics()
	}

	return &Analyzer{
		headings:             make(map[string]int),
		links:                []string{},
		hc:                   hc,
		mc:                   m,
		TaskStatusUpdateFunc: func(taskType types.TaskType, status types.TaskStatus) {},
		AddSubTaskFunc:       func(taskType types.TaskType, key, url string) {},
		SubTaskUpdateFunc:    func(taskType types.TaskType, key string, subtask types.SubTask) {},
	}
}

func (a *Analyzer) AnalyzeHTML(ctx context.Context, content string) (types.AnalyzeResult, error) {
	doc, err := a.parseHTML(content)
	if err != nil {
		return types.AnalyzeResult{}, errors.Join(err, errors.New("failed to parse HTML"))
	}

	a.htmlVersion = a.detectHtmlVersion(content)
	a.analyzeContent(doc)
	a.verifyLinks(ctx)

	return types.AnalyzeResult{
		HtmlVersion:       a.htmlVersion,
		PageTitle:         a.title,
		Headings:          a.headings,
		Links:             a.links,
		InternalLinkCount: int(atomic.LoadInt32(&a.internalLinks)),
		ExternalLinkCount: int(atomic.LoadInt32(&a.externalLinks)),
		AccessibleLinks:   int(atomic.LoadInt32(&a.accessibleLinks)),
		InaccessibleLinks: int(atomic.LoadInt32(&a.inaccessibleLinks)),
		HasLoginForm:      a.hasLoginForm,
	}, nil
}

// SetBaseUrl sets the base URL for resolving relative links
func (a *Analyzer) SetBaseUrl(baseUrl string) {
	a.baseUrl = baseUrl
}

func (a *Analyzer) parseHTML(content string) (doc *html.Node, err error) {
	taskStart := time.Now()
	a.TaskStatusUpdateFunc(types.TaskTypeExtracting, types.TaskStatusPending)

	defer func() {
		if err != nil {
			a.TaskStatusUpdateFunc(types.TaskTypeExtracting, types.TaskStatusFailed)
			a.mc.RecordAnalysisTask(string(types.TaskTypeExtracting), false, time.Since(taskStart).Seconds())
		} else {
			a.TaskStatusUpdateFunc(types.TaskTypeExtracting, types.TaskStatusCompleted)
			a.mc.RecordAnalysisTask(string(types.TaskTypeExtracting), true, time.Since(taskStart).Seconds())
		}
	}()

	doc, err = html.Parse(strings.NewReader(content))
	if err != nil {
		return nil, errors.Join(err, errors.New("failed to parse HTML"))
	}

	return doc, nil
}

func (a *Analyzer) detectHtmlVersion(content string) string {
	taskStart := time.Now()
	a.TaskStatusUpdateFunc(types.TaskTypeIdentifyingVersion, types.TaskStatusRunning)
	defer func() {
		a.TaskStatusUpdateFunc(types.TaskTypeIdentifyingVersion, types.TaskStatusCompleted)
		a.mc.RecordAnalysisTask(string(types.TaskTypeIdentifyingVersion), true, time.Since(taskStart).Seconds())
	}()

	content = strings.ToLower(content)

	// HTML5 variations
	if strings.Contains(content, "<!doctype html>") ||
		strings.Contains(content, "<!doctype html ") {
		return "HTML5"
	}

	// HTML 4.01 Strict
	if strings.Contains(content, `"-//w3c//dtd html 4.01//en"`) {
		return "HTML 4.01 Strict"
	}

	// HTML 4.01 Transitional
	if strings.Contains(content, `"-//w3c//dtd html 4.01 transitional//en"`) {
		return "HTML 4.01 Transitional"
	}

	// XHTML 1.0 Strict
	if strings.Contains(content, `"-//w3c//dtd xhtml 1.0 strict//en"`) {
		return "XHTML 1.0 Strict"
	}

	// XHTML 1.0 Transitional
	if strings.Contains(content, `"-//w3c//dtd xhtml 1.0 transitional//en"`) {
		return "XHTML 1.0 Transitional"
	}

	// Check for XML declaration (XHTML)
	if strings.HasPrefix(strings.TrimSpace(content), "<?xml") {
		return "XHTML (XML Declaration)"
	}

	return "No DOCTYPE or Unrecognized"
}

func (a *Analyzer) analyzeContent(doc *html.Node) {
	taskStart := time.Now()
	a.TaskStatusUpdateFunc(types.TaskTypeAnalyzing, types.TaskStatusRunning)
	defer func() {
		a.TaskStatusUpdateFunc(types.TaskTypeAnalyzing, types.TaskStatusCompleted)
		a.mc.RecordAnalysisTask(string(types.TaskTypeAnalyzing), true, time.Since(taskStart).Seconds())
	}()

	a.dfs(doc)
}

func (a *Analyzer) dfs(n *html.Node) {
	if n.Type == html.ElementNode {
		switch n.Data {
		case "title":
			if n.FirstChild != nil {
				a.title = strings.TrimSpace(n.FirstChild.Data)
			}
		case "h1", "h2", "h3", "h4", "h5", "h6":
			a.headings[n.Data]++
		case "a":
			for _, attr := range n.Attr {
				if attr.Key == "href" && attr.Val != "" {
					href := attr.Val
					if a.shouldProcessLink(href) {
						// Handle relative URLs by resolving them against the base URL
						resolvedURL := a.resolveURL(href)
						if resolvedURL != "" {
							a.links = append(a.links, resolvedURL)

							if a.isExternalURL(resolvedURL) {
								atomic.AddInt32(&a.externalLinks, 1)
							} else {
								atomic.AddInt32(&a.internalLinks, 1)
							}
						}
					}
				}
			}
		case "form":
			if a.isLoginForm(n) {
				a.hasLoginForm = true
			}
		}
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		a.dfs(c)
	}
}

// resolveURL resolves a relative URL to an absolute URL and returns the resolved URL
func (a *Analyzer) resolveURL(href string) string {
	// Already absolute URL
	if a.isAbsoluteURL(href) {
		return href
	}

	// Need base URL to resolve relative URLs
	if a.baseUrl == "" {
		logger.Warn("Cannot resolve relative URL without base URL", slog.String("href", href))
		return ""
	}

	base, err := url.Parse(a.baseUrl)
	if err != nil {
		logger.Error("Failed to parse base URL", slog.String("baseURL", a.baseUrl), slog.Any("error", err))
		return ""
	}

	relativeURL, err := url.Parse(href)
	if err != nil {
		logger.Error("Failed to parse relative URL", slog.String("href", href), slog.Any("error", err))
		return ""
	}

	resolvedURL := base.ResolveReference(relativeURL)
	return resolvedURL.String()
}

// isAbsoluteURL checks if a URL is absolute
func (a *Analyzer) isAbsoluteURL(href string) bool {
	return strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://")
}

// isExternalURL determines if a URL is external to the base domain
func (a *Analyzer) isExternalURL(absoluteURL string) bool {
	// If no base URL is set, assume external
	if a.baseUrl == "" {
		return true
	}

	targetURL, err := url.Parse(absoluteURL)
	if err != nil {
		logger.Error("Failed to parse target URL for external check",
			slog.String("url", absoluteURL), slog.Any("error", err))
		return true // Assume external on parse error
	}

	baseURL, err := url.Parse(a.baseUrl)
	if err != nil {
		logger.Error("Failed to parse base URL for external check",
			slog.String("baseURL", a.baseUrl), slog.Any("error", err))
		return true // Assume external on parse error
	}

	// Same scheme and host: internal
	if targetURL.Scheme == baseURL.Scheme && targetURL.Host == baseURL.Host {
		return false
	}

	// Different schemes: external
	if targetURL.Scheme != baseURL.Scheme {
		return true
	}

	// Check if targetURL is a subdomain of baseURL or vice versa
	return !a.isSubdomainOf(targetURL.Host, baseURL.Host)
}

// isSubdomainOf checks if host1 is a subdomain of host2 or vice versa
func (a *Analyzer) isSubdomainOf(host1, host2 string) bool {
	// Remove ports for comparison
	host1 = a.stripPort(host1)
	host2 = a.stripPort(host2)

	// Exact match
	if host1 == host2 {
		return true
	}

	// Check if one is a subdomain of the other
	// e.g., "api.example.com" is a subdomain of "example.com"
	// or "example.com" is the parent of "api.example.com"
	if strings.HasSuffix(host1, "."+host2) || strings.HasSuffix(host2, "."+host1) {
		return true
	}

	return false
}

// stripPort removes port number from host if present
func (a *Analyzer) stripPort(host string) string {
	if colonIndex := strings.LastIndex(host, ":"); colonIndex != -1 {
		// Check if this is actually a port (IPv6 addresses have colons too)
		if portPart := host[colonIndex+1:]; portPart != "" {
			// If it's all digits, it's likely a port
			for _, r := range portPart {
				if r < '0' || r > '9' {
					return host // Not a port, return host
				}
			}
			return host[:colonIndex] // Strip the port and return host
		}
	}
	return host
}

func (a *Analyzer) shouldProcessLink(href string) bool {
	if href == "" || href == "/" {
		return false
	}

	if strings.HasPrefix(href, "#") {
		return false
	}

	if strings.HasPrefix(href, "javascript:") {
		return false
	}

	if strings.HasPrefix(href, "mailto:") {
		return false
	}

	if strings.HasPrefix(href, "tel:") {
		return false
	}

	if strings.HasPrefix(href, "data:") {
		return false
	}

	if strings.HasPrefix(href, "about:") {
		return false
	}

	return true
}

// getAttr extracts attribute values from HTML nodes
func (a *Analyzer) getAttr(n *html.Node, key string) string {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

// isUsernameField checks if an input field is likely a username/email field
func (a *Analyzer) isUsernameField(inputType, name, id, placeholder string) bool {
	// Convert to lowercase for case-insensitive comparison
	inputType = strings.ToLower(inputType)
	name = strings.ToLower(name)
	id = strings.ToLower(id)
	placeholder = strings.ToLower(placeholder)

	if inputType == "email" {
		return true
	}

	// For text type, check name, id, and placeholder for username indicators
	if inputType == "text" || inputType == "" {
		usernameKeywords := []string{
			"user", "username", "login", "email", "account", "signin", "sign-in",
		}

		for _, keyword := range usernameKeywords {
			if strings.Contains(name, keyword) ||
				strings.Contains(id, keyword) ||
				strings.Contains(placeholder, keyword) {
				return true
			}
		}
	}

	return false
}

func (a *Analyzer) isLoginForm(formNode *html.Node) bool {
	hasPasswordField := false
	hasUsernameField := false
	hasSubmitButton := false

	a.dfsFormInputs(formNode, &hasPasswordField, &hasUsernameField, &hasSubmitButton)

	// All three components are required for a login form
	return hasPasswordField && hasUsernameField && hasSubmitButton
}

func (a *Analyzer) dfsFormInputs(n *html.Node, hasPassword, hasUsername, hasSubmit *bool) {
	if n.Type == html.ElementNode {
		switch n.Data {
		case "input":
			inputType := strings.ToLower(a.getAttr(n, "type"))
			name := a.getAttr(n, "name")
			id := a.getAttr(n, "id")
			placeholder := a.getAttr(n, "placeholder")

			switch inputType {
			case "password":
				*hasPassword = true
			case "submit":
				*hasSubmit = true
			default:
				// Check if this is a username field (email, text with username-like attributes)
				if a.isUsernameField(inputType, name, id, placeholder) {
					*hasUsername = true
				}
			}
		case "button":
			buttonType := strings.ToLower(a.getAttr(n, "type"))
			// Button elements with type="submit" or no type (defaults to submit in forms)
			if buttonType == "submit" || (buttonType == "" && n.Parent != nil) {
				*hasSubmit = true
			}
		}
	}

	// Recursively check child nodes
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		a.dfsFormInputs(c, hasPassword, hasUsername, hasSubmit)
		// Early exit when all required components are found
		if *hasPassword && *hasUsername && *hasSubmit {
			break
		}
	}
}

func (a *Analyzer) verifyLinks(ctx context.Context) {
	taskStart := time.Now()
	a.TaskStatusUpdateFunc(types.TaskTypeVerifyingLinks, types.TaskStatusRunning)
	defer func() {
		a.TaskStatusUpdateFunc(types.TaskTypeVerifyingLinks, types.TaskStatusCompleted)
		a.mc.RecordAnalysisTask(string(types.TaskTypeVerifyingLinks), true, time.Since(taskStart).Seconds())
	}()

	linkCount := len(a.links)
	if linkCount == 0 {
		return
	}

	logger.Info("Starting link verification", slog.Int("linkCount", linkCount))

	// Track concurrent link verifications
	a.mc.SetConcurrentLinkVerifications(linkCount)
	defer a.mc.SetConcurrentLinkVerifications(0)

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, maxConc)

	for i, link := range a.links {
		key := strconv.Itoa(i + 1)
		a.AddSubTaskFunc(types.TaskTypeVerifyingLinks, key, link)

		logger.Debug("Added subtask for link verification",
			slog.String("key", key),
			slog.String("url", link))

		wg.Add(1)
		go func(ctx context.Context, link, key string) {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() {
				<-semaphore
			}()

			a.SubTaskUpdateFunc(types.TaskTypeVerifyingLinks, key, types.SubTask{
				Type:   types.SubTaskTypeValidatingLink,
				Status: types.TaskStatusRunning,
				URL:    link,
			})

			linkVerifyStart := time.Now()
			status, description := a.verifyLink(ctx, link)
			linkVerifyDuration := time.Since(linkVerifyStart).Seconds()

			a.SubTaskUpdateFunc(types.TaskTypeVerifyingLinks, key, types.SubTask{
				Type:        types.SubTaskTypeValidatingLink,
				Status:      status,
				URL:         link,
				Description: description,
			})

			if status == types.TaskStatusCompleted {
				atomic.AddInt32(&a.accessibleLinks, 1)
			} else {
				atomic.AddInt32(&a.inaccessibleLinks, 1)
			}

			a.mc.RecordLinkVerification(status == types.TaskStatusCompleted, linkVerifyDuration)

		}(ctx, link, key)
	}

	wg.Wait()
	logger.Info("Completed link verification", slog.Int("linkCount", linkCount))
}

func (a *Analyzer) verifyLink(ctx context.Context, link string) (types.TaskStatus, string) {
	parsedURL, err := url.Parse(link)
	if err != nil {
		errMsg := fmt.Sprintf("Invalid URL: %s", err.Error())
		logger.Error("Error parsing URL",
			slog.String("url", link),
			slog.Any("error", err))

		return types.TaskStatusFailed, errMsg
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		description := fmt.Sprintf("Unsupported protocol: %s", parsedURL.Scheme)
		logger.Debug("Skipping non-HTTP URL",
			slog.String("url", link),
			slog.String("scheme", parsedURL.Scheme))
		return types.TaskStatusSkipped, description
	}

	// Start with HEAD request
	status, description, shouldRetryWithGET := a.tryHEADRequest(ctx, link)

	// If HEAD failed with specific errors that suggest GET might work, retry with GET
	if shouldRetryWithGET {
		logger.Debug("Retrying with GET request",
			slog.String("url", link),
			slog.String("reason", "HEAD request failed or not supported"))
		status, description = a.tryGETRequest(ctx, link)
	}

	return status, description
}

// tryHEADRequest attempts to verify a link using HEAD request
func (a *Analyzer) tryHEADRequest(ctx context.Context, link string) (types.TaskStatus, string, bool) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, link, nil)
	if err != nil {
		errMsg := fmt.Sprintf("HEAD request creation failed: %s", err.Error())
		logger.Error("Failed to create HEAD request",
			slog.String("url", link),
			slog.Any("error", err))
		return types.TaskStatusFailed, errMsg, false
	}

	start := time.Now()
	resp, err := a.hc.Do(req)
	if err != nil {
		errMsg := a.formatRequestError(err)
		logger.Debug("HEAD request failed",
			slog.String("url", link),
			slog.Any("error", err))
		a.mc.RecordHTTPClientRequest(0, time.Since(start).Seconds(), http.MethodHead, "link_verification")
		return types.TaskStatusFailed, errMsg, false
	}
	defer resp.Body.Close()

	a.mc.RecordHTTPClientRequest(resp.StatusCode, time.Since(start).Seconds(), http.MethodHead, "link_verification")

	// Check if we should retry with GET
	shouldRetryWithGET := a.shouldRetryWithGET(resp.StatusCode)

	if shouldRetryWithGET {
		return types.TaskStatusPending, "HEAD not supported, retrying with GET", true
	}

	// Process successful HEAD response
	description := a.formatResponse(resp)

	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		logger.Debug("Link verified with HEAD",
			slog.String("url", link),
			slog.Int("statusCode", resp.StatusCode))
		return types.TaskStatusCompleted, description, false
	}

	logger.Debug("Link verification failed with HEAD",
		slog.String("url", link),
		slog.Int("statusCode", resp.StatusCode))
	return types.TaskStatusFailed, description, false
}

// tryGETRequest attempts to verify a link using GET request (fallback)
func (a *Analyzer) tryGETRequest(ctx context.Context, link string) (types.TaskStatus, string) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, link, nil)
	if err != nil {
		errMsg := fmt.Sprintf("GET request creation failed: %s", err.Error())
		logger.Error("Failed to create GET request",
			slog.String("url", link),
			slog.Any("error", err))
		return types.TaskStatusFailed, errMsg
	}

	start := time.Now()
	resp, err := a.hc.Do(req)
	if err != nil {
		errMsg := a.formatRequestError(err)
		logger.Error("GET request failed",
			slog.String("url", link),
			slog.Any("error", err))
		a.mc.RecordHTTPClientRequest(0, time.Since(start).Seconds(), http.MethodGet, "link_verification")
		return types.TaskStatusFailed, errMsg
	}
	defer resp.Body.Close()

	a.mc.RecordHTTPClientRequest(resp.StatusCode, time.Since(start).Seconds(), http.MethodGet, "link_verification")

	description := a.formatResponse(resp)

	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		logger.Debug("Link verified with GET",
			slog.String("url", link),
			slog.Int("statusCode", resp.StatusCode))
		return types.TaskStatusCompleted, description
	}

	logger.Warn("Link verification failed with GET",
		slog.String("url", link),
		slog.Int("statusCode", resp.StatusCode))
	return types.TaskStatusFailed, description
}

// shouldRetryWithGET determines if we should retry a failed HEAD request with GET
func (a *Analyzer) shouldRetryWithGET(statusCode int) bool {
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
func (a *Analyzer) formatRequestError(err error) string {
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
func (a *Analyzer) formatResponse(resp *http.Response) string {
	description := fmt.Sprintf("HTTP %d: %s", resp.StatusCode, http.StatusText(resp.StatusCode))

	// Check for redirects
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		if location, ok := resp.Header["Location"]; ok && len(location) > 0 {
			description = fmt.Sprintf("HTTP %d: Redirected to %s", resp.StatusCode, location[0])
		}
	}

	return description
}
