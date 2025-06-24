package main

import (
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"shared/types"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"golang.org/x/net/html"
)

const (
	maxConc = 10
)

type TaskStatusUpdateCallback func(taskType types.TaskType, status types.TaskStatus)
type AddSubTaskCallback func(taskType types.TaskType, key, url string)
type SubTaskStatusUpdateCallback func(taskType types.TaskType, key string, status types.TaskStatus)

type Analyzer struct {
	htmlVersion       string
	title             string
	headings          map[string]int
	links             []string
	internalLinks     int
	externalLinks     int
	accessibleLinks   atomic.Int32
	inaccessibleLinks atomic.Int32
	hasLoginForm      bool
	baseUrl           string

	hc *http.Client

	TaskStatusUpdateCallback    TaskStatusUpdateCallback
	AddSubTaskCallback          AddSubTaskCallback
	SubTaskStatusUpdateCallback SubTaskStatusUpdateCallback
}

func NewAnalyzer() *Analyzer {
	return &Analyzer{
		headings:                    make(map[string]int),
		links:                       []string{},
		hc:                          &http.Client{Timeout: httpClientTimeout},
		TaskStatusUpdateCallback:    func(taskType types.TaskType, status types.TaskStatus) {},
		AddSubTaskCallback:          func(taskType types.TaskType, key, url string) {},
		SubTaskStatusUpdateCallback: func(taskType types.TaskType, key string, status types.TaskStatus) {},
	}
}

func (a *Analyzer) AnalyzeHTML(content string) (types.AnalyzeResult, error) {
	a.TaskStatusUpdateCallback(types.TaskTypeExtracting, types.TaskStatusPending)
	doc, err := html.Parse(strings.NewReader(content))
	if err != nil {
		a.TaskStatusUpdateCallback(types.TaskTypeExtracting, types.TaskStatusFailed)
		return types.AnalyzeResult{}, errors.Join(err, errors.New("failed to parse HTML"))
	}
	a.TaskStatusUpdateCallback(types.TaskTypeExtracting, types.TaskStatusCompleted)

	a.TaskStatusUpdateCallback(types.TaskTypeIdentifyingVersion, types.TaskStatusRunning)
	a.htmlVersion = a.detectHtmlVersion(content)
	a.TaskStatusUpdateCallback(types.TaskTypeIdentifyingVersion, types.TaskStatusCompleted)

	a.TaskStatusUpdateCallback(types.TaskTypeAnalyzing, types.TaskStatusRunning)
	a.dfs(doc)
	a.TaskStatusUpdateCallback(types.TaskTypeAnalyzing, types.TaskStatusCompleted)

	a.TaskStatusUpdateCallback(types.TaskTypeVerifyingLinks, types.TaskStatusRunning)
	a.verifyLinks()
	a.TaskStatusUpdateCallback(types.TaskTypeVerifyingLinks, types.TaskStatusCompleted)

	return types.AnalyzeResult{
		HtmlVersion:       a.htmlVersion,
		PageTitle:         a.title,
		Headings:          a.headings,
		Links:             a.links,
		InternalLinkCount: a.internalLinks,
		ExternalLinkCount: a.externalLinks,
		AccessibleLinks:   int(a.accessibleLinks.Load()),
		InaccessibleLinks: int(a.inaccessibleLinks.Load()),
		HasLoginForm:      a.hasLoginForm,
	}, nil
}

// SetBaseUrl sets the base URL for resolving relative links
func (a *Analyzer) SetBaseUrl(baseUrl string) {
	a.baseUrl = baseUrl
}

func (a *Analyzer) detectHtmlVersion(content string) string {
	content = strings.ToLower(content)

	if strings.Contains(content, "<!doctype html>") {
		return "HTML5"
	}

	return "No !doctype"
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
								a.externalLinks++
							} else {
								a.internalLinks++
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

func (a *Analyzer) verifyLinks() {
	linkCount := len(a.links)
	if linkCount == 0 {
		return
	}

	logger.Info("Starting link verification", slog.Int("linkCount", linkCount))

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, maxConc)

	for i, link := range a.links {
		key := strconv.Itoa(i + 1)
		a.AddSubTaskCallback(types.TaskTypeVerifyingLinks, key, link)

		logger.Debug("Added subtask for link verification",
			slog.String("key", key),
			slog.String("url", link))

		wg.Add(1)
		go func(link, key string) {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			a.SubTaskStatusUpdateCallback(types.TaskTypeVerifyingLinks, key, types.TaskStatusRunning)
			status := a.verifyLink(link)
			a.SubTaskStatusUpdateCallback(types.TaskTypeVerifyingLinks, key, status)

			if status == types.TaskStatusCompleted {
				a.accessibleLinks.Add(1)
			} else {
				a.inaccessibleLinks.Add(1)
			}

		}(link, key)
	}

	wg.Wait()
	logger.Info("Completed link verification", slog.Int("linkCount", linkCount))
}

func (a *Analyzer) verifyLink(link string) types.TaskStatus {
	parsedURL, err := url.Parse(link)
	if err != nil {
		logger.Error("Error parsing URL",
			slog.String("url", link),
			slog.Any("error", err))
		return types.TaskStatusFailed
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		logger.Debug("Skipping non-HTTP URL",
			slog.String("url", link),
			slog.String("scheme", parsedURL.Scheme))
		return types.TaskStatusSkipped
	}

	req, err := http.NewRequest(http.MethodHead, link, nil)
	if err != nil {
		logger.Error("Failed to create request",
			slog.String("url", link),
			slog.Any("error", err))
		return types.TaskStatusFailed
	}

	resp, err := a.hc.Do(req)
	if err != nil {
		logger.Error("Failed to verify link",
			slog.String("url", link),
			slog.Any("error", err))
		return types.TaskStatusFailed
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		logger.Debug("Link verified",
			slog.String("url", link),
			slog.Int("statusCode", resp.StatusCode))
		return types.TaskStatusCompleted
	}

	logger.Warn("Link verification failed",
		slog.String("url", link),
		slog.Int("statusCode", resp.StatusCode))
	return types.TaskStatusFailed
}
