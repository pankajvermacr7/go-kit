package http

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"
)

// APIClient represents the HTTP client.
type APIClient struct {
	BaseURL    string
	HTTPClient *http.Client
	token      string
}

// APIClientConfig holds the configuration values for the API client.
type APIClientConfig struct {
	MaxIdleConns          int
	MaxIdleConnsPerHost   int
	MaxConnsPerHost       int
	IdleConnTimeout       time.Duration
	TLSHandshakeTimeout   time.Duration
	ExpectContinueTimeout time.Duration
	DisableKeepAlives     bool
	DialTimeout           time.Duration
	DialKeepAlive         time.Duration
	RequestTimeout        time.Duration
}

// DefaultAPIClientConfig returns the default configuration for the API client.
func DefaultAPIClientConfig() APIClientConfig {
	return APIClientConfig{
		MaxIdleConns:          10000,
		MaxIdleConnsPerHost:   10000,
		MaxConnsPerHost:       10000,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		DisableKeepAlives:     false,
		DialTimeout:           30 * time.Second,
		DialKeepAlive:         30 * time.Second,
		RequestTimeout:        30 * time.Second,
	}
}

// NewAPIClient initializes a new API client.
func NewAPIClient(baseURL string, token string, config APIClientConfig) *APIClient {
	// Use default configuration if not provided
	if config == (APIClientConfig{}) {
		config = DefaultAPIClientConfig()
	}
	transport := &http.Transport{
		// Connection Pool Settings
		MaxIdleConns:          config.MaxIdleConns,
		MaxIdleConnsPerHost:   config.MaxIdleConnsPerHost,
		MaxConnsPerHost:       config.MaxConnsPerHost,
		IdleConnTimeout:       config.IdleConnTimeout,
		TLSHandshakeTimeout:   config.TLSHandshakeTimeout,
		ExpectContinueTimeout: config.ExpectContinueTimeout,
		DisableKeepAlives:     config.DisableKeepAlives,

		// DNS Caching
		DialContext: (&net.Dialer{
			Timeout:   config.DialTimeout,
			KeepAlive: config.DialKeepAlive,
		}).DialContext,
	}

	return &APIClient{
		HTTPClient: &http.Client{
			Timeout:   config.RequestTimeout, // Global timeout for requests
			Transport: transport,
		},
		BaseURL: baseURL,
		token:   token,
	}
}

// RequestParams defines the input parameters for making an API request.
type RequestParams struct {
	Method      string            // HTTP method (GET, POST, PUT, etc.)
	Path        string            // API endpoint path
	Headers     map[string]string // HTTP headers
	QueryParams map[string]string // Query parameters
	Body        interface{}       // Request body
}

// APIResponse represents a generic API response.
type APIResponse struct {
	StatusCode int         // HTTP status code
	Headers    http.Header // Response headers
	Body       []byte      // Raw response body
}

// MakeRequest performs an HTTP request and returns the response.
func (c *APIClient) MakeRequest(params RequestParams) (*APIResponse, error) {
	// Build the URL with query parameters
	fullURL, err := c.buildURL(params.Path, params.QueryParams)
	if err != nil {
		return nil, fmt.Errorf("failed to build URL: %w", err)
	}

	// Marshal the body (if present)
	var requestBody []byte
	if params.Body != nil {
		requestBody, err = json.Marshal(params.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal body: %w", err)
		}
	}

	// Create the HTTP request
	req, err := http.NewRequest(params.Method, fullURL, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	for key, value := range params.Headers {
		req.Header.Set(key, value)
	}

	// Perform the request
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read the response body
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Return the response
	return &APIResponse{
		StatusCode: resp.StatusCode,
		Headers:    resp.Header,
		Body:       responseBody,
	}, nil
}

// buildURL constructs the full URL with query parameters.
func (c *APIClient) buildURL(path string, queryParams map[string]string) (string, error) {
	base, err := url.Parse(c.BaseURL)
	if err != nil {
		return "", err
	}

	endpoint, err := url.Parse(path)
	if err != nil {
		return "", err
	}

	fullURL := base.ResolveReference(endpoint)

	// Add query parameters
	query := fullURL.Query()
	for key, value := range queryParams {
		query.Add(key, value)
	}
	fullURL.RawQuery = query.Encode()

	return fullURL.String(), nil
}
