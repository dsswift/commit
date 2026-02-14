// Package httpclient provides a shared HTTP transport with connection pooling.
package httpclient

import (
	"net"
	"net/http"
	"time"
)

// sharedTransport is a shared HTTP transport with connection pooling.
var sharedTransport = &http.Transport{
	MaxIdleConns:        20,
	MaxIdleConnsPerHost: 5,
	IdleConnTimeout:     90 * time.Second,
	DialContext: (&net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}).DialContext,
	TLSHandshakeTimeout: 10 * time.Second,
}

// NewClient creates an HTTP client using the shared transport with the given timeout.
func NewClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout:   timeout,
		Transport: sharedTransport,
	}
}
