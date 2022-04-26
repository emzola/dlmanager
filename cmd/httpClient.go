package cmd

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

// httpClient creates an HTTP client.
func httpClient() *http.Client {
	// redirectPolicyFunc does not follow redirection request
	redirectPolicyFunc := func(r *http.Request, via []*http.Request) error {
		if len(via) >= 1 {
			return errors.New("stopped after 1 redirect")
		}
		return nil
	}

	// Configure the connection pool
	t := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          25,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	return &http.Client{
		CheckRedirect: redirectPolicyFunc,
		Transport:     t,
	}
}

// sendHTTPRequest sends an HTTP request and returns a response.
func sendHTTPRequest(url string, client *http.Client) (*http.Response, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// sendHTTPRequestWithHeader sends an HTTP request with range header and returns a response.
func sendHTTPRequestWithHeader(url string, client *http.Client, fileSize int64) (*http.Response, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	// Set range header to the request if file already exists at destination path
	if fileSize > 0 && fileSize != req.ContentLength {
		req.Header.Set("Range", fmt.Sprintf("bytes=%v-", fileSize))
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// sendHTTPHeadRequest sends an HTTP HEAD request and returns a response.
func sendHTTPHeadRequest(url string, client *http.Client) (*http.Response, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodHead, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}