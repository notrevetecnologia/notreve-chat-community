package webhooks

import (
	"crypto/tls"
	"net/http"
)

// NewSecureOutboundHTTPClient cria o cliente usado por workers de webhook.
// Ele não usa proxy, só aceita TLS 1.2+ e não segue redirects: uma mudança de
// destino deve ser validada e aprovada como uma nova URL, nunca seguida de
// forma implícita pelo processo de entrega.
func NewSecureOutboundHTTPClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = safeDialContext

	tlsConfig := transport.TLSClientConfig
	if tlsConfig == nil {
		tlsConfig = &tls.Config{}
	} else {
		tlsConfig = tlsConfig.Clone()
	}
	if tlsConfig.MinVersion < tls.VersionTLS12 {
		tlsConfig.MinVersion = tls.VersionTLS12
	}
	transport.TLSClientConfig = tlsConfig

	return &http.Client{
		Transport: transport,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}
