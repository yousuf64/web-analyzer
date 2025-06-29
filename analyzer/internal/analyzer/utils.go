package analyzer

import (
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

// getElementAttribute extracts attribute values from HTML nodes
func (s *Analyzer) getElementAttribute(n *html.Node, key string) string {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

// resolveURL resolves a relative URL to an absolute URL
func (s *Analyzer) resolveURL(href, baseURL string) string {
	// Already absolute URL
	if s.isAbsoluteURL(href) {
		return href
	}

	// Need base URL to resolve relative URLs
	if baseURL == "" {
		s.log.Warn("Cannot resolve relative URL without base URL", "href", href)
		return ""
	}

	base, err := url.Parse(baseURL)
	if err != nil {
		s.log.Error("Failed to parse base URL", "baseURL", baseURL, "error", err)
		return ""
	}

	relativeURL, err := url.Parse(href)
	if err != nil {
		s.log.Error("Failed to parse relative URL", "href", href, "error", err)
		return ""
	}

	resolvedURL := base.ResolveReference(relativeURL)
	return resolvedURL.String()
}

// isAbsoluteURL checks if a URL is absolute
func (s *Analyzer) isAbsoluteURL(href string) bool {
	return strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://")
}

// isExternalURL determines if a URL is external to the base domain
func (s *Analyzer) isExternalURL(absoluteURL, baseURL string) bool {
	// If no base URL is set, assume external
	if baseURL == "" {
		return true
	}

	targetURL, err := url.Parse(absoluteURL)
	if err != nil {
		s.log.Error("Failed to parse target URL for external check", "url", absoluteURL, "error", err)
		return true // Assume external on parse error
	}

	baseURLParsed, err := url.Parse(baseURL)
	if err != nil {
		s.log.Error("Failed to parse base URL for external check", "baseURL", baseURL, "error", err)
		return true // Assume external on parse error
	}

	// Same scheme and host: internal
	if targetURL.Scheme == baseURLParsed.Scheme && targetURL.Host == baseURLParsed.Host {
		return false
	}

	// Different schemes: external
	if targetURL.Scheme != baseURLParsed.Scheme {
		return true
	}

	return true
}

// shouldProcessLink determines if a link should be processed
func (s *Analyzer) shouldProcessLink(href string) bool {
	if href == "" || href == "/" {
		return false
	}

	excludedPrefixes := []string{
		"#", "javascript:", "mailto:", "tel:", "data:", "about:",
	}

	for _, prefix := range excludedPrefixes {
		if strings.HasPrefix(href, prefix) {
			return false
		}
	}

	return true
}

// isLoginForm checks if a form is a login form
func (s *Analyzer) isLoginForm(formNode *html.Node) bool {
	hasPasswordField := false
	hasUsernameField := false
	hasSubmitButton := false

	s.traverseFormInputs(formNode, &hasPasswordField, &hasUsernameField, &hasSubmitButton)

	// All three components are required for a login form
	return hasPasswordField && hasUsernameField && hasSubmitButton
}

// traverseFormInputs traverses form inputs to detect login form characteristics
func (s *Analyzer) traverseFormInputs(n *html.Node, hasPassword, hasUsername, hasSubmit *bool) {
	if n.Type == html.ElementNode {
		switch n.Data {
		case "input":
			s.processInputElement(n, hasPassword, hasUsername, hasSubmit)
		case "button":
			s.processButtonElement(n, hasSubmit)
		}
	}

	// Recursively check child nodes
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		s.traverseFormInputs(c, hasPassword, hasUsername, hasSubmit)
		// Early exit when all required components are found
		if *hasPassword && *hasUsername && *hasSubmit {
			break
		}
	}
}

// processInputElement processes input elements for login form detection
func (s *Analyzer) processInputElement(n *html.Node, hasPassword, hasUsername, hasSubmit *bool) {
	inputType := strings.ToLower(s.getElementAttribute(n, "type"))
	name := s.getElementAttribute(n, "name")
	id := s.getElementAttribute(n, "id")
	placeholder := s.getElementAttribute(n, "placeholder")

	switch inputType {
	case "password":
		*hasPassword = true
	case "submit":
		*hasSubmit = true
	default:
		// Check if this is a username field (email, text with username-like attributes)
		if s.isUsernameField(inputType, name, id, placeholder) {
			*hasUsername = true
		}
	}
}

// processButtonElement processes button elements for login form detection
func (s *Analyzer) processButtonElement(n *html.Node, hasSubmit *bool) {
	buttonType := strings.ToLower(s.getElementAttribute(n, "type"))
	// Button elements with type="submit" or no type (defaults to submit in forms)
	if buttonType == "submit" || (buttonType == "" && n.Parent != nil) {
		*hasSubmit = true
	}
}

// isUsernameField checks if an input field is likely a username/email field
func (s *Analyzer) isUsernameField(inputType, name, id, placeholder string) bool {
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
