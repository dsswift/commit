package httpclient

import (
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	client := NewClient(30 * time.Second)
	if client == nil {
		t.Fatal("expected non-nil client")
	}
	if client.Timeout != 30*time.Second {
		t.Errorf("expected timeout 30s, got %v", client.Timeout)
	}
	if client.Transport != sharedTransport {
		t.Error("expected shared transport")
	}
}

func TestNewClient_DifferentTimeouts(t *testing.T) {
	c1 := NewClient(5 * time.Second)
	c2 := NewClient(60 * time.Second)

	if c1.Timeout == c2.Timeout {
		t.Error("clients with different timeouts should have different timeouts")
	}

	// Both should share the same transport
	if c1.Transport != c2.Transport {
		t.Error("clients should share the same transport")
	}
}
