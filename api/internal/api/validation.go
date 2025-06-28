package api

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"
)

// validHostnameRegex is a regular expression to validate hostnames
var validHostnameRegex = regexp.MustCompile(`^([a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?\.)*[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?$`)

// validateURL validates the URL
func validateURL(rawURL string) (string, error) {
	if rawURL == "" {
		return "", errors.New("url is required")
	}

	rawURL = strings.TrimSpace(rawURL)

	if len(rawURL) > 2048 {
		return "", errors.New("url too long (max 2048 characters)")
	}

	if !strings.Contains(rawURL, "://") {
		rawURL = "https://" + rawURL
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid url format: %w", err)
	}

	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", fmt.Errorf("unsupported scheme '%s': only http and https are allowed", u.Scheme)
	}

	if u.Host == "" {
		return "", errors.New("hostname is required")
	}

	hostname := u.Hostname()
	if hostname == "" {
		return "", errors.New("invalid hostname")
	}

	if err := validateHostname(hostname); err != nil {
		return "", fmt.Errorf("invalid hostname: %w", err)
	}

	if strings.Contains(u.Path, "..") {
		return "", errors.New("path traversal patterns are not allowed")
	}

	return u.String(), nil
}

// validateHostname validates the hostname
func validateHostname(hostname string) error {
	if isLocalhost(hostname) {
		return errors.New("localhost and loopback addresses are not allowed")
	}

	if isPrivateIP(hostname) {
		return errors.New("private IP addresses are not allowed")
	}

	if !validHostnameRegex.MatchString(hostname) {
		if net.ParseIP(hostname) == nil {
			return errors.New("invalid hostname or IP address format")
		}
	}

	if len(hostname) > 253 {
		return errors.New("hostname too long (max 253 characters)")
	}

	return nil
}

// isLocalhost checks if the hostname is a localhost address
func isLocalhost(hostname string) bool {
	localhost := []string{"localhost", "127.0.0.1", "::1", "0.0.0.0"}
	hostname = strings.ToLower(hostname)
	for _, local := range localhost {
		if hostname == local {
			return true
		}
	}
	return strings.HasSuffix(hostname, ".localhost")
}

// isPrivateIP checks if the hostname is a private IP address
func isPrivateIP(hostname string) bool {
	ip := net.ParseIP(hostname)
	if ip == nil {
		return false
	}

	privateRanges := []string{
		"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16",
		"169.254.0.0/16", "fc00::/7", "fe80::/10",
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
