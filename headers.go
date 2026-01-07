package main

import (
	"net/url"
	"strings"
)

// generateHeadersForDomain generates domain-specific headers
func generateHeadersForDomain(targetURL *url.URL) map[string]string {
	headers := map[string]string{
		"User-Agent":      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
		"Accept":          "*/*",
		"Accept-Language": "en-US,en;q=0.9",
		"Connection":      "keep-alive",
	}

	hostname := strings.ToLower(targetURL.Hostname())

	// Add domain-specific headers
	if strings.Contains(hostname, "example.com") {
		headers["Referer"] = targetURL.Scheme + "://" + targetURL.Host + "/"
	}

	// Add Origin header for certain domains
	if strings.Contains(hostname, "cdn") || strings.Contains(hostname, "stream") {
		headers["Origin"] = targetURL.Scheme + "://" + targetURL.Host
	}

	return headers
}

// generateRequestHeaders generates request headers with optional overrides
func generateRequestHeaders(targetURL string, additionalHeaders map[string]string) map[string]string {
	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		// Use default headers if URL parsing fails
		headers := map[string]string{
			"User-Agent":      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
			"Accept":          "*/*",
			"Accept-Language": "en-US,en;q=0.9",
			"Connection":      "keep-alive",
		}
		// Merge additional headers
		for k, v := range additionalHeaders {
			if v != "" {
				headers[k] = v
			}
		}
		return headers
	}

	// Generate base headers for the domain
	headers := generateHeadersForDomain(parsedURL)

	// Merge additional headers (they override base headers)
	for k, v := range additionalHeaders {
		if v != "" {
			headers[k] = v
		}
	}

	return headers
}
