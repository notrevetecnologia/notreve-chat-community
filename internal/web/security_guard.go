package web

import (
	"net/http"
	"net/url"
	"strings"
)

func unsafeHTTPMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func requestOriginMatchesHost(r *http.Request, raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return false
	}
	return strings.EqualFold(u.Host, r.Host)
}

// strictSameOriginRequest é fail-closed. Requisições autenticadas de mutação
// precisam comprovar a origem pelo Origin ou Referer; a ausência de ambos não
// é tratada como autorização implícita.
func strictSameOriginRequest(r *http.Request) bool {
	if origin := r.Header.Get("Origin"); origin != "" {
		return requestOriginMatchesHost(r, origin)
	}
	if referer := r.Header.Get("Referer"); referer != "" {
		return requestOriginMatchesHost(r, referer)
	}
	return false
}

func setSaaSSecurityHeaders(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "SAMEORIGIN")
	w.Header().Set("Referrer-Policy", "same-origin")
	w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; base-uri 'self'; form-action 'self'; frame-ancestors 'self'; object-src 'none'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; font-src 'self' data:; connect-src 'self'")
	if r.TLS != nil || strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")), "https") {
		w.Header().Set("Strict-Transport-Security", "max-age=31536000")
	}
}

func hasAuthenticatedPanelCookie(r *http.Request) bool {
	cookie, err := r.Cookie(sessionCookie)
	return err == nil && strings.TrimSpace(cookie.Value) != ""
}
