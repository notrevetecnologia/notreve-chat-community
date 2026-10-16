package channel

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/notrevetecnologia/notreve-chat-community/internal/securesecret"
)

type SecretAwareTransport struct {
	Base http.RoundTripper
}

func (t SecretAwareTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}
	value := strings.TrimSpace(req.Header.Get("token"))
	if !securesecret.IsEncrypted(value) {
		return base.RoundTrip(req)
	}
	plain, err := securesecret.Decrypt("channels.token", value)
	if err != nil {
		return nil, errors.New("credencial protegida do canal indisponível")
	}
	clone := req.Clone(req.Context())
	clone.Header = req.Header.Clone()
	clone.Header.Set("token", plain)
	return base.RoundTrip(clone)
}

func NewSecretAwareTransport(base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	if _, ok := base.(SecretAwareTransport); ok {
		return base
	}
	return SecretAwareTransport{Base: base}
}

var secretAwareHTTPTransport = NewSecretAwareTransport(http.DefaultTransport)
var secretAwareHTTPClient = &http.Client{Transport: secretAwareHTTPTransport}

// NewPrivateBlockedHTTPClient cria um cliente HTTP que rejeita conexões a
// endereços privados/loopback/link-local (anti-SSRF) e não segue redirects.
// Usado ao baixar mídia de URLs fornecidas pelo webhook, que não são fonte
// confiável e não devem alcançar hosts internos.
func NewPrivateBlockedHTTPClient() *http.Client {
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           safeDialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          10,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	return &http.Client{Transport: transport, Timeout: 60 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}
}

var mediaDownloadClient = NewPrivateBlockedHTTPClient()

func isBlockedHost(host string) bool {
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") || strings.HasSuffix(host, ".home.arpa") || strings.HasSuffix(host, ".lan") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && (ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() || ip.IsMulticast())
}

func safeDialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return nil, err
	}
	for _, ip := range ips {
		if isBlockedHost(ip.String()) {
			return nil, fmt.Errorf("endereço privado não permitido")
		}
	}
	dialer := &net.Dialer{Timeout: 15 * time.Second}
	var lastErr error
	for _, ip := range ips {
		conn, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if dialErr == nil {
			return conn, nil
		}
		lastErr = dialErr
	}
	return nil, lastErr
}
