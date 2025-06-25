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

type TaskStatusUpdateCallback func(taskType types.TaskType, status types.TaskStatus)
type AddSubTaskCallback func(taskType types.TaskType, key, url string)
type SubTaskUpdateCallback func(taskType types.TaskType, key string, subtask types.SubTask)

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

	TaskStatusUpdateCallback TaskStatusUpdateCallback
	AddSubTaskCallback       AddSubTaskCallback
	SubTaskUpdateCallback    SubTaskUpdateCallback
}

func NewAnalyzer(hc *http.Client, m metrics.AnalyzerMetricsInterface) *Analyzer {
	if m == nil {
		m = metrics.NewNoopAnalyzerMetrics()
	}

	return &Analyzer{
		headings:                 make(map[string]int),
		links:                    []string{},
		hc:                       hc,
		mc:                       m,
		TaskStatusUpdateCallback: func(taskType types.TaskType, status types.TaskStatus) {},
		AddSubTaskCallback:       func(taskType types.TaskType, key, url string) {},
		SubTaskUpdateCallback:    func(taskType types.TaskType, key string, subtask types.SubTask) {},
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
	a.TaskStatusUpdateCallback(types.TaskTypeExtracting, types.TaskStatusPending)

	defer func() {
		if err != nil {
			a.TaskStatusUpdateCallback(types.TaskTypeExtracting, types.TaskStatusFailed)
			a.mc.RecordAnalysisTask(string(types.TaskTypeExtracting), false, time.Since(taskStart).Seconds())
		} else {
			a.TaskStatusUpdateCallback(types.TaskTypeExtracting, types.TaskStatusCompleted)
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
	a.TaskStatusUpdateCallback(types.TaskTypeIdentifyingVersion, types.TaskStatusRunning)
	defer func() {
		a.TaskStatusUpdateCallback(types.TaskTypeIdentifyingVersion, types.TaskStatusCompleted)
		a.mc.RecordAnalysisTask(string(types.TaskTypeIdentifyingVersion), true, time.Since(taskStart).Seconds())
	}()

	content = strings.ToLower(content)

	if strings.Contains(content, "<!doctype html>") {
		return "HTML5"
	}

	return "No !doctype"
}

func (a *Analyzer) analyzeContent(doc *html.Node) {
	taskStart := time.Now()
	a.TaskStatusUpdateCallback(types.TaskTypeAnalyzing, types.TaskStatusRunning)
	defer func() {
		a.TaskStatusUpdateCallback(types.TaskTypeAnalyzing, types.TaskStatusCompleted)
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
						resolvedURL, isExternal := a.resolveURL(href)
						if resolvedURL != "" {
							a.links = append(a.links, resolvedURL)

							if isExternal {
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

// resolveURL resolves a relative URL to an absolute URL and returns true if the URL is external
func (a *Analyzer) resolveURL(href string) (string, bool) {
	// Absolute URL, no need to resolve
	if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
		return href, true
	}

	if a.baseUrl == "" {
		logger.Warn("Cannot resolve relative URL without base URL", slog.String("href", href))
		return "", false
	}

	base, err := url.Parse(a.baseUrl)
	if err != nil {
		logger.Error("Failed to parse base URL", slog.String("baseURL", a.baseUrl), slog.Any("error", err))
		return "", false
	}

	relativeURL, err := url.Parse(href)
	if err != nil {
		logger.Error("Failed to parse relative URL", slog.String("href", href), slog.Any("error", err))
		return "", false
	}

	resolvedURL := base.ResolveReference(relativeURL)
	return resolvedURL.String(), false
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

func (a *Analyzer) isLoginForm(formNode *html.Node) bool {
	hasPasswordField := false
	hasUsernameField := false

	a.dfsFormInputs(formNode, &hasPasswordField, &hasUsernameField)

	return hasPasswordField && hasUsernameField
}

func (a *Analyzer) dfsFormInputs(n *html.Node, hasPassword, hasEmail *bool) {
	if n.Type == html.ElementNode && n.Data == "input" {
		inputType := ""

		for _, attr := range n.Attr {
			if attr.Key == "type" {
				inputType = strings.ToLower(attr.Val)
			}
		}

		if inputType == "password" {
			*hasPassword = true
		}

		if inputType == "email" {
			*hasEmail = true
		}
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		a.dfsFormInputs(c, hasPassword, hasEmail)
		if *hasPassword && *hasEmail {
			break
		}
	}
}

func (a *Analyzer) verifyLinks(ctx context.Context) {
	taskStart := time.Now()
	a.TaskStatusUpdateCallback(types.TaskTypeVerifyingLinks, types.TaskStatusRunning)
	defer func() {
		a.TaskStatusUpdateCallback(types.TaskTypeVerifyingLinks, types.TaskStatusCompleted)
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
		a.AddSubTaskCallback(types.TaskTypeVerifyingLinks, key, link)

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

			a.SubTaskUpdateCallback(types.TaskTypeVerifyingLinks, key, types.SubTask{
				Type:   types.SubTaskTypeValidatingLink,
				Status: types.TaskStatusRunning,
				URL:    link,
			})

			linkVerifyStart := time.Now()
			status, description := a.verifyLink(ctx, link)
			linkVerifyDuration := time.Since(linkVerifyStart).Seconds()

			a.SubTaskUpdateCallback(types.TaskTypeVerifyingLinks, key, types.SubTask{
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

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, link, nil)
	if err != nil {
		errMsg := fmt.Sprintf("Request creation failed: %s", err.Error())
		logger.Error("Failed to create request",
			slog.String("url", link),
			slog.Any("error", err))

		return types.TaskStatusFailed, errMsg
	}

	start := time.Now()
	resp, err := a.hc.Do(req)
	if err != nil {
		// URL error, connection failed, etc.
		var errMsg string
		if urlErr, ok := err.(*url.Error); ok {
			if urlErr.Timeout() {
				errMsg = "Connection timeout"
			} else {
				errMsg = fmt.Sprintf("Connection error: %s", urlErr.Err.Error())
			}
		} else {
			errMsg = fmt.Sprintf("Request failed: %s", err.Error())
		}

		logger.Error("Failed to verify link",
			slog.String("url", link),
			slog.Any("error", err))

		a.mc.RecordHTTPClientRequest(0, time.Since(start).Seconds(), http.MethodHead, "link_verification")
		return types.TaskStatusFailed, errMsg
	}
	defer resp.Body.Close()

	a.mc.RecordHTTPClientRequest(resp.StatusCode, time.Since(start).Seconds(), http.MethodHead, "link_verification")

	description := fmt.Sprintf("HTTP %d: %s", resp.StatusCode, http.StatusText(resp.StatusCode))

	// Check for redirects
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		if location, ok := resp.Header["Location"]; ok && len(location) > 0 {
			description = fmt.Sprintf("HTTP %d: Redirected to %s", resp.StatusCode, location[0])
		}
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		logger.Debug("Link verified",
			slog.String("url", link),
			slog.Int("statusCode", resp.StatusCode))
		return types.TaskStatusCompleted, description
	}

	logger.Warn("Link verification failed",
		slog.String("url", link),
		slog.Int("statusCode", resp.StatusCode))
	return types.TaskStatusFailed, description
}
