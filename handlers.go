package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
)

var sharedClient = &http.Client{
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return fmt.Errorf("stopped after 5 redirects")
		}
		return nil
	},
}

// resolveURL resolves a relative URL against a base URL
func resolveURL(href, base string) string {
	baseURL, err := url.Parse(base)
	if err != nil {
		return href
	}

	relURL, err := url.Parse(href)
	if err != nil {
		return href
	}

	return baseURL.ResolveReference(relURL).String()
}

// validateRequest validates and extracts URL and headers from request
func validateRequest(r *http.Request) (string, map[string]string, error) {
	targetURL := r.URL.Query().Get("url")
	if targetURL == "" {
		return "", nil, fmt.Errorf("URL parameter is required")
	}

	parsedHeaders := make(map[string]string)
	headersParam := r.URL.Query().Get("headers")
	if headersParam != "" {
		decodedHeaders, err := url.QueryUnescape(headersParam)
		if err == nil {
			json.Unmarshal([]byte(decodedHeaders), &parsedHeaders)
		}
	}

	return targetURL, parsedHeaders, nil
}

// sendError sends an error response
func sendError(w http.ResponseWriter, message string, details interface{}) {
	log.Printf("%s: %v", message, details)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error":   message,
		"details": details,
	})
}

// m3u8ProxyHandler handles M3U8 playlist proxying
func m3u8ProxyHandler(w http.ResponseWriter, r *http.Request) {
	targetURL, parsedHeaders, err := validateRequest(r)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	requestHeaders := generateRequestHeaders(targetURL, parsedHeaders)

	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		sendError(w, "Failed to create request", err.Error())
		return
	}

	for k, v := range requestHeaders {
		req.Header.Set(k, v)
	}

	resp, err := sharedClient.Do(req)
	if err != nil {
		sendError(w, "Failed to proxy m3u8 content", err.Error())
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		sendError(w, "Failed to read m3u8 content", err.Error())
		return
	}

	m3u8Content := string(body)
	lines := strings.Split(m3u8Content, "\n")
	newLines := make([]string, 0, len(lines))

	// Encode headers for URL parameters
	headersJSON, _ := json.Marshal(requestHeaders)
	encodedHeaders := url.QueryEscape(string(headersJSON))

	segmentCount := 0

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if strings.HasPrefix(trimmedLine, "#") {
			// Handle URI in tags (e.g., encryption keys, I-Frames)
			if strings.Contains(line, "URI=") {
				if start := strings.Index(line, `URI="`); start != -1 {
					start += 5 // len(`URI="`)
					if end := strings.Index(line[start:], `"`); end != -1 {
						originalURI := line[start : start+end]
						resolvedKeyURL := resolveURL(originalURI, targetURL)

						var currentBaseURL string
						// Check if the URI is a playlist (e.g., I-Frame playlist)
						if strings.HasSuffix(strings.ToLower(resolvedKeyURL), ".m3u8") {
							currentBaseURL = webServerURLs[0]
						} else {
							currentBaseURL = webServerURLs[segmentCount%len(webServerURLs)]
							segmentCount++
						}

						newURI := fmt.Sprintf("%s/ts-proxy?url=%s&headers=%s",
							currentBaseURL,
							url.QueryEscape(resolvedKeyURL),
							encodedHeaders)
						line = strings.Replace(line, originalURI, newURI, 1)
					}
				}
			}
			newLines = append(newLines, line)
		} else if trimmedLine != "" {
			// Trim the line to ensure clean URL resolution
			resolvedURL := resolveURL(trimmedLine, targetURL)
			var newURL string

			// Check if the resolved URL ends with .m3u8 (variant playlist)
			if strings.HasSuffix(strings.ToLower(resolvedURL), ".m3u8") {
				// Use primary URL for playlists, no rotation
				newURL = fmt.Sprintf("%s/proxy?url=%s&headers=%s",
					webServerURLs[0],
					url.QueryEscape(resolvedURL),
					encodedHeaders)
			} else {
				// For all other files (segments, keys, etc.), use round-robin ts-proxy
				currentBaseURL := webServerURLs[segmentCount%len(webServerURLs)]
				segmentCount++

				newURL = fmt.Sprintf("%s/ts-proxy?url=%s&headers=%s",
					currentBaseURL,
					url.QueryEscape(resolvedURL),
					encodedHeaders)
			}
			newLines = append(newLines, newURL)
		} else {
			newLines = append(newLines, line)
		}
	}

	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	w.Write([]byte(strings.Join(newLines, "\n")))
}

// tsProxyHandler handles TS segment and general content proxying
func tsProxyHandler(w http.ResponseWriter, r *http.Request) {
	targetURL, parsedHeaders, err := validateRequest(r)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	requestHeaders := generateRequestHeaders(targetURL, parsedHeaders)

	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		sendError(w, "Failed to create request", err.Error())
		return
	}

	for k, v := range requestHeaders {
		req.Header.Set(k, v)
	}

	resp, err := sharedClient.Do(req)
	if err != nil {
		sendError(w, "Failed to proxy segment", err.Error())
		return
	}
	defer resp.Body.Close()

	// Determine content type
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		if strings.HasSuffix(targetURL, ".ts") {
			contentType = "video/mp2t"
		} else if strings.HasSuffix(targetURL, ".m3u8") {
			contentType = "application/vnd.apple.mpegurl"
		} else if strings.Contains(targetURL, ".jpg") || strings.Contains(targetURL, ".jpeg") ||
			strings.Contains(targetURL, ".png") || strings.Contains(targetURL, ".gif") ||
			strings.Contains(targetURL, ".webp") || strings.Contains(targetURL, ".bmp") ||
			strings.Contains(targetURL, ".svg") {
			contentType = "image/jpeg"
		} else {
			contentType = "application/octet-stream"
		}
	}

	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(resp.StatusCode)

	io.Copy(w, resp.Body)
}

// mp4ProxyHandler handles MP4 video proxying with range support
func mp4ProxyHandler(w http.ResponseWriter, r *http.Request) {
	targetURL, parsedHeaders, err := validateRequest(r)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// Forward Range header if provided by the client
	rangeHeader := r.Header.Get("Range")
	if rangeHeader != "" {
		parsedHeaders["Range"] = rangeHeader
	}

	requestHeaders := generateRequestHeaders(targetURL, parsedHeaders)

	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		sendError(w, "Failed to create request", err.Error())
		return
	}

	for k, v := range requestHeaders {
		req.Header.Set(k, v)
	}

	resp, err := sharedClient.Do(req)
	if err != nil {
		sendError(w, "Failed to proxy mp4 content", err.Error())
		return
	}
	defer resp.Body.Close()

	// Set CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Range")

	// Use upstream headers when available
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "video/mp4"
	}
	w.Header().Set("Content-Type", contentType)

	if contentLength := resp.Header.Get("Content-Length"); contentLength != "" {
		w.Header().Set("Content-Length", contentLength)
	}

	if contentRange := resp.Header.Get("Content-Range"); contentRange != "" {
		w.Header().Set("Content-Range", contentRange)
	}

	acceptRanges := resp.Header.Get("Accept-Ranges")
	if acceptRanges == "" {
		acceptRanges = "bytes"
	}
	w.Header().Set("Accept-Ranges", acceptRanges)
	w.Header().Set("Content-Disposition", "inline")

	w.WriteHeader(resp.StatusCode)

	io.Copy(w, resp.Body)
}

// fetchHandler handles generic fetch requests with optional referer and custom headers
func fetchHandler(w http.ResponseWriter, r *http.Request) {
	targetURL := r.URL.Query().Get("url")
	if targetURL == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "URL parameter is required"})
		return
	}

	// Optional referer convenience param
	referer := r.URL.Query().Get("ref")

	// Optional header overrides via `headers` query param (URL-escaped JSON)
	parsedHeaders := make(map[string]string)
	if headersParam := r.URL.Query().Get("headers"); headersParam != "" {
		if decoded, err := url.QueryUnescape(headersParam); err == nil {
			_ = json.Unmarshal([]byte(decoded), &parsedHeaders)
		}
	}
	if referer != "" {
		parsedHeaders["Referer"] = referer
	}
	// Forward Range from client if present and not overridden
	if rangeHeader := r.Header.Get("Range"); rangeHeader != "" {
		if _, exists := parsedHeaders["Range"]; !exists {
			parsedHeaders["Range"] = rangeHeader
		}
	}

	// Generate headers tailored to the target domain, allowing overrides
	requestHeaders := generateRequestHeaders(targetURL, parsedHeaders)

	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Request failed",
			"error":   err.Error(),
		})
		return
	}

	for k, v := range requestHeaders {
		if v != "" {
			req.Header.Set(k, v)
		}
	}

	resp, err := sharedClient.Do(req)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Request failed",
			"error":   err.Error(),
		})
		return
	}
	defer resp.Body.Close()

	// Propagate upstream content headers when useful
	if contentType := resp.Header.Get("Content-Type"); contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	if contentLength := resp.Header.Get("Content-Length"); contentLength != "" {
		w.Header().Set("Content-Length", contentLength)
	}
	if contentRange := resp.Header.Get("Content-Range"); contentRange != "" {
		w.Header().Set("Content-Range", contentRange)
	}
	if acceptRanges := resp.Header.Get("Accept-Ranges"); acceptRanges != "" {
		w.Header().Set("Accept-Ranges", acceptRanges)
	}

	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// videostrProxyHandler handles requests with videostr.net specific headers
// URL format: http://localhost:3000/{url_without_https}
func videostrProxyHandler(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")

	if path == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "URL path is required"})
		return
	}

	// Construct the full URL with https://
	targetURL := "https://" + path
	if r.URL.RawQuery != "" {
		targetURL += "?" + r.URL.RawQuery
	}

	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Request failed",
			"error":   err.Error(),
		})
		return
	}

	// Set videostr.net specific headers
	req.Header.Set("Referer", "https://videostr.net/")
	req.Header.Set("Origin", "https://videostr.net/")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	req.Header.Set("Accept", "*/*")

	resp, err := sharedClient.Do(req)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Request failed",
			"error":   err.Error(),
		})
		return
	}
	defer resp.Body.Close()

	contentType := resp.Header.Get("Content-Type")
	isM3U8 := strings.Contains(contentType, "mpegurl") || strings.HasSuffix(path, ".m3u8")

	if isM3U8 {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		lines := strings.Split(string(body), "\n")
		newLines := make([]string, 0, len(lines))
		segmentCount := 0

		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(line, "#") {
				if strings.Contains(line, "URI=") {
					if start := strings.Index(line, `URI="`); start != -1 {
						start += 5
						if end := strings.Index(line[start:], `"`); end != -1 {
							originalURI := line[start : start+end]
							resolvedKeyURL := resolveURL(originalURI, targetURL)
							proxyPath := strings.TrimPrefix(strings.TrimPrefix(resolvedKeyURL, "https://"), "http://")

							var currentBaseURL string
							if strings.HasSuffix(strings.ToLower(resolvedKeyURL), ".m3u8") {
								currentBaseURL = webServerURLs[0]
							} else {
								currentBaseURL = webServerURLs[segmentCount%len(webServerURLs)]
								segmentCount++
							}

							newURI := currentBaseURL + "/" + proxyPath
							line = strings.Replace(line, originalURI, newURI, 1)
						}
					}
				}
				newLines = append(newLines, line)
			} else if trimmed != "" {
				resolvedURL := resolveURL(line, targetURL)
				proxyPath := strings.TrimPrefix(strings.TrimPrefix(resolvedURL, "https://"), "http://")

				if strings.HasSuffix(strings.ToLower(resolvedURL), ".m3u8") {
					// Use primary URL for playlists
					newLines = append(newLines, webServerURLs[0]+"/"+proxyPath)
				} else {
					// Use round-robin for segments
					currentBaseURL := webServerURLs[segmentCount%len(webServerURLs)]
					segmentCount++
					newLines = append(newLines, currentBaseURL+"/"+proxyPath)
				}
			} else {
				newLines = append(newLines, line)
			}
		}

		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		w.Write([]byte(strings.Join(newLines, "\n")))
	} else {
		// Stream non-M3U8 content directly
		if contentType != "" {
			w.Header().Set("Content-Type", contentType)
		}
		if contentLength := resp.Header.Get("Content-Length"); contentLength != "" {
			w.Header().Set("Content-Length", contentLength)
		}
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
	}
}
