package analyzer

import (
	"context"
	"fmt"
	"shared/models"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/net/html"
)

// analyzeHTML performs complete HTML analysis
func (s *Analyzer) analyzeHTML(ctx context.Context, jobID, content string, result *AnalysisResult) error {
	doc, err := s.parseHTML(ctx, jobID, content)
	if err != nil {
		return fmt.Errorf("failed to parse HTML: %w", err)
	}

	s.detectHTMLVersion(ctx, jobID, content, result)
	s.analyzeContent(ctx, jobID, doc, result)
	s.verifyLinks(ctx, jobID, result)

	return nil
}

// parseHTML parses HTML content and tracks the parsing task
func (s *Analyzer) parseHTML(ctx context.Context, jobID, content string) (*html.Node, error) {
	start := time.Now()
	s.updateTaskStatus(ctx, jobID, models.TaskTypeExtracting, models.TaskStatusPending)

	doc, err := html.Parse(strings.NewReader(content))

	success := err == nil
	s.metrics.RecordAnalysisTask(string(models.TaskTypeExtracting), success, time.Since(start).Seconds())

	if err != nil {
		s.updateTaskStatus(ctx, jobID, models.TaskTypeExtracting, models.TaskStatusFailed)
		return nil, fmt.Errorf("HTML parsing failed: %w", err)
	}

	s.updateTaskStatus(ctx, jobID, models.TaskTypeExtracting, models.TaskStatusCompleted)
	return doc, nil
}

// detectHTMLVersion identifies the HTML version from the document
func (s *Analyzer) detectHTMLVersion(ctx context.Context, jobID, content string, result *AnalysisResult) {
	start := time.Now()
	s.updateTaskStatus(ctx, jobID, models.TaskTypeIdentifyingVersion, models.TaskStatusRunning)

	defer func() {
		s.updateTaskStatus(ctx, jobID, models.TaskTypeIdentifyingVersion, models.TaskStatusCompleted)
		s.metrics.RecordAnalysisTask(string(models.TaskTypeIdentifyingVersion), true, time.Since(start).Seconds())
	}()

	content = strings.ToLower(content)
	result.htmlVersion = s.parseHTMLVersion(content)
}

// parseHTMLVersion parses HTML version from DOCTYPE declaration
func (s *Analyzer) parseHTMLVersion(content string) string {
	// Check for XML declaration (XHTML) first
	if strings.HasPrefix(strings.TrimSpace(content), "<?xml") {
		return "XHTML (XML Declaration)"
	}

	// HTML 4.01 Strict (check before HTML5 to avoid false positives)
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

	// HTML5 variations (check more specifically)
	if strings.Contains(content, "<!doctype html>") ||
		(strings.Contains(content, "<!doctype html ") && !strings.Contains(content, "public")) {
		return "HTML5"
	}

	return "No DOCTYPE or Unrecognized"
}

// analyzeContent performs content analysis using DFS traversal
func (s *Analyzer) analyzeContent(ctx context.Context, jobID string, doc *html.Node, result *AnalysisResult) {
	start := time.Now()
	s.updateTaskStatus(ctx, jobID, models.TaskTypeAnalyzing, models.TaskStatusRunning)

	defer func() {
		s.updateTaskStatus(ctx, jobID, models.TaskTypeAnalyzing, models.TaskStatusCompleted)
		s.metrics.RecordAnalysisTask(string(models.TaskTypeAnalyzing), true, time.Since(start).Seconds())
	}()

	s.traverseNode(doc, result)
}

// traverseNode performs depth-first traversal of HTML nodes
func (s *Analyzer) traverseNode(n *html.Node, result *AnalysisResult) {
	if n.Type == html.ElementNode {
		s.processElement(n, result)
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		s.traverseNode(c, result)
	}
}

// processElement processes different HTML elements
func (s *Analyzer) processElement(n *html.Node, result *AnalysisResult) {
	switch n.Data {
	case "title":
		s.extractTitle(n, result)
	case "h1", "h2", "h3", "h4", "h5", "h6":
		s.extractHeading(n, result)
	case "a":
		s.extractLink(n, result)
	case "form":
		s.checkLoginForm(n, result)
	}
}

// extractTitle extracts the page title
func (s *Analyzer) extractTitle(n *html.Node, result *AnalysisResult) {
	if n.FirstChild != nil {
		result.title = strings.TrimSpace(n.FirstChild.Data)
	}
}

// extractHeading counts heading elements
func (s *Analyzer) extractHeading(n *html.Node, result *AnalysisResult) {
	result.headings[n.Data]++
}

// extractLink processes anchor elements
func (s *Analyzer) extractLink(n *html.Node, result *AnalysisResult) {
	href := s.getElementAttribute(n, "href")
	if href == "" || !s.shouldProcessLink(href) {
		return
	}

	resolvedURL := s.resolveURL(href, result.baseURL)
	if resolvedURL == "" {
		return
	}

	result.links = append(result.links, resolvedURL)

	if s.isExternalURL(resolvedURL, result.baseURL) {
		atomic.AddInt32(&result.externalLinks, 1)
	} else {
		atomic.AddInt32(&result.internalLinks, 1)
	}
}

// checkLoginForm checks if a form is a login form
func (s *Analyzer) checkLoginForm(n *html.Node, result *AnalysisResult) {
	if s.isLoginForm(n) {
		result.hasLoginForm = true
	}
}

// buildResult builds and returns the analysis result
func (s *Analyzer) buildResult(result *AnalysisResult) models.AnalyzeResult {
	return models.AnalyzeResult{
		HtmlVersion:       result.htmlVersion,
		PageTitle:         result.title,
		Headings:          result.headings,
		Links:             result.links,
		InternalLinkCount: int(atomic.LoadInt32(&result.internalLinks)),
		ExternalLinkCount: int(atomic.LoadInt32(&result.externalLinks)),
		AccessibleLinks:   int(atomic.LoadInt32(&result.accessibleLinks)),
		InaccessibleLinks: int(atomic.LoadInt32(&result.inaccessibleLinks)),
		HasLoginForm:      result.hasLoginForm,
	}
}
