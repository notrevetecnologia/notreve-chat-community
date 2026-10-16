package webhooks

import (
	"crypto/tls"
	"net/http"
	"testing"
)

func TestNewSecureOutboundHTTPClientDisablesRedirectsAndProxy(t *testing.T) {
	client := NewSecureOutboundHTTPClient()
	if client.CheckRedirect == nil {
		t.Fatal("secure client must define redirect policy")
	}
	if err := client.CheckRedirect(&http.Request{}, nil); err != http.ErrUseLastResponse {
		t.Fatalf("redirect error = %v", err)
	}
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport = %T, want *http.Transport", client.Transport)
	}
	if transport.Proxy != nil {
		t.Fatal("secure client must not use a proxy")
	}
	if transport.TLSClientConfig == nil || transport.TLSClientConfig.MinVersion < tls.VersionTLS12 {
		t.Fatal("secure client must require TLS 1.2 or newer")
	}
}
