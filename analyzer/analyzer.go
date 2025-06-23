package main

import (
	"context"
	"log"
	"net/http"
	"net/url"
	"shared/types"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
)

type Analyzer struct {
	htmlVersion           string
	title                 string
	headings              map[string]int
	links                 []string
	hasLoginForm          bool
	onTaskStatusUpdate    func(taskType types.TaskType, status types.TaskStatus)
	onSubTaskStatusUpdate func(taskType types.TaskType, key string, status types.TaskStatus)
	onAddSubTask          func(taskType types.TaskType, key, url string) // Takes the key as parameter
}

func NewAnalyzer(
	onTaskStatusUpdate func(taskType types.TaskType, status types.TaskStatus),
	onSubTaskStatusUpdate func(taskType types.TaskType, key string, status types.TaskStatus),
	onAddSubTask func(taskType types.TaskType, key, url string)) *Analyzer {
	return &Analyzer{
		headings:              make(map[string]int),
		links:                 []string{},
		onTaskStatusUpdate:    onTaskStatusUpdate,
		onSubTaskStatusUpdate: onSubTaskStatusUpdate,
		onAddSubTask:          onAddSubTask,
	}
}

func (a *Analyzer) AnalyzeHTML(content string) (types.AnalyzeResult, error) {
	a.onTaskStatusUpdate(types.TaskTypeExtracting, types.TaskStatusPending)
	doc, err := html.Parse(strings.NewReader(content))
	if err != nil {
		log.Printf("Failed to parse HTML: %v", err)
		a.onTaskStatusUpdate(types.TaskTypeExtracting, types.TaskStatusFailed)
		return types.AnalyzeResult{}, err
	}
	a.onTaskStatusUpdate(types.TaskTypeExtracting, types.TaskStatusCompleted)

	a.onTaskStatusUpdate(types.TaskTypeIdentifyingVersion, types.TaskStatusRunning)
	a.htmlVersion = a.detectHtmlVersion(content)
	a.onTaskStatusUpdate(types.TaskTypeIdentifyingVersion, types.TaskStatusCompleted)

	a.onTaskStatusUpdate(types.TaskTypeAnalyzing, types.TaskStatusRunning)
	a.dfs(doc)
	a.onTaskStatusUpdate(types.TaskTypeAnalyzing, types.TaskStatusCompleted)

	a.onTaskStatusUpdate(types.TaskTypeVerifyingLinks, types.TaskStatusRunning)
	a.verifyLinks()
	a.onTaskStatusUpdate(types.TaskTypeVerifyingLinks, types.TaskStatusCompleted)

	return types.AnalyzeResult{
		HtmlVersion:  a.htmlVersion,
		PageTitle:    a.title,
		Headings:     a.headings,
		Links:        a.links,
		HasLoginForm: a.hasLoginForm,
	}, nil
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
					a.links = append(a.links, attr.Val)
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
	log.Printf("Starting link verification for %d links", len(a.links))

	if len(a.links) == 0 {
		return
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	var wg sync.WaitGroup

	maxConcurrency := 10
	semaphore := make(chan struct{}, maxConcurrency)

	for i, link := range a.links {
		key := strconv.Itoa(i + 1)
		a.onAddSubTask(types.TaskTypeVerifyingLinks, key, link)
		log.Printf("Added subtask for link verification: %s with key: %s", link, key)

		wg.Add(1)
		go func(link, key string) {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			a.onSubTaskStatusUpdate(types.TaskTypeVerifyingLinks, key, types.TaskStatusRunning)

			status := a.verifyLink(client, link)

			a.onSubTaskStatusUpdate(types.TaskTypeVerifyingLinks, key, status)
		}(link, key)
	}

	wg.Wait()
	log.Printf("Completed link verification for %d links", len(a.links))
}

func (a *Analyzer) verifyLink(client *http.Client, link string) types.TaskStatus {
	parsedURL, err := url.Parse(link)
	if err != nil {
		log.Printf("Error parsing URL: %s, error: %v", link, err)
		return types.TaskStatusFailed
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		log.Printf("Skipping non-HTTP URL: %s", link)
		return types.TaskStatusSkipped
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, link, nil)
	if err != nil {
		log.Printf("Failed to create request %s: %v", link, err)
		return types.TaskStatusFailed
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Failed to verify link %s: %v", link, err)
		return types.TaskStatusFailed
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		log.Printf("Link verified: %s, status: %d", link, resp.StatusCode)
		return types.TaskStatusCompleted
	}

	log.Printf("Link verification failed: %s, status: %d", link, resp.StatusCode)
	return types.TaskStatusFailed
}