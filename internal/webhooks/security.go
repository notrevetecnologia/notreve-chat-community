package webhooks

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func Sign(secret []byte, timestamp time.Time, body []byte) (string, error) {
	if err := validateSecret(secret); err != nil {
		return "", err
	}
	message := signingMessage(timestamp.Unix(), body)
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(message))
	return "v1=" + hex.EncodeToString(mac.Sum(nil)), nil
}

func Verify(secret []byte, timestampHeader string, body []byte, signature string, now time.Time, tolerance time.Duration) error {
	if err := validateSecret(secret); err != nil {
		return err
	}
	if tolerance < 0 {
		return errors.New("tolerância de timestamp inválida")
	}
	timestamp, err := strconv.ParseInt(strings.TrimSpace(timestampHeader), 10, 64)
	if err != nil {
		return errors.New("timestamp inválido")
	}
	if tolerance > 0 && absDuration(now.Sub(time.Unix(timestamp, 0))) > tolerance {
		return errors.New("timestamp fora da janela de tolerância")
	}
	expected, err := Sign(secret, time.Unix(timestamp, 0), body)
	if err != nil {
		return err
	}
	if !hmac.Equal([]byte(expected), []byte(strings.TrimSpace(signature))) {
		return errors.New("assinatura inválida")
	}
	return nil
}

func PayloadHash(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func signingMessage(timestamp int64, body []byte) string {
	return strconv.FormatInt(timestamp, 10) + "." + string(body)
}

func validateSecret(secret []byte) error {
	if len(secret) < 16 || len(secret) > 256 {
		return errors.New("segredo deve ter entre 16 e 256 bytes")
	}
	return nil
}

func absDuration(value time.Duration) time.Duration {
	if value < 0 {
		return -value
	}
	return value
}

func ValidateURL(rawURL string, allowedHosts []string) error {
	parsed, err := url.ParseRequestURI(strings.TrimSpace(rawURL))
	if err != nil || parsed.Hostname() == "" || strings.ToLower(parsed.Scheme) != "https" {
		return errors.New("webhook deve usar uma URL HTTPS válida")
	}
	if parsed.User != nil || parsed.Fragment != "" {
		return errors.New("url do webhook não pode conter credenciais ou fragmento")
	}
	host := strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
	if isBlockedHost(host) {
		return errors.New("host privado não permitido")
	}
	for _, rule := range allowedHosts {
		if err := validateHostRule(rule); err != nil {
			return err
		}
	}
	if len(allowedHosts) > 0 && !hostAllowed(host, allowedHosts) {
		return errors.New("host não está no escopo autorizado")
	}
	return nil
}

func validateHostRule(rawRule string) error {
	rule := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(rawRule), "."))
	if rule == "" || strings.ContainsAny(rule, "/\\?#:@\r\n") {
		return errors.New("host autorizado inválido")
	}
	if len(rule) > 253 {
		return errors.New("host autorizado inválido")
	}
	if strings.HasPrefix(rule, "*.") {
		rule = strings.TrimPrefix(rule, "*.")
	}
	if ip := net.ParseIP(rule); ip != nil {
		if isBlockedHost(ip.String()) {
			return errors.New("host autorizado inválido")
		}
		return nil
	}
	if !hostnamePattern.MatchString(rule) {
		return errors.New("host autorizado inválido")
	}
	return nil
}

var hostnamePattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+$`)

func hostAllowed(host string, allowedHosts []string) bool {
	for _, rawRule := range allowedHosts {
		rule := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(rawRule), "."))
		if rule == host || strings.HasPrefix(rule, "*.") && strings.HasSuffix(host, "."+strings.TrimPrefix(rule, "*.")) {
			return true
		}
	}
	return false
}

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
