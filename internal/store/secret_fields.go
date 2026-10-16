package store

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/notrevetecnologia/notreve-chat-community/internal/securesecret"
)

func encryptSettingsSecrets(st Settings) (Settings, error) {
	fields := []struct {
		scope string
		ptr   *string
	}{
		{"settings.default_api_key", &st.DefaultAPIKey},
		{"settings.whatsapp_token", &st.WhatsAppToken},
		{"settings.whatsapp_admin_token", &st.WhatsAppAdminToken},
		{"settings.stt_api_key", &st.STTAPIKey},
	}
	for _, field := range fields {
		value, err := securesecret.Encrypt(field.scope, *field.ptr)
		if err != nil {
			return Settings{}, fmt.Errorf("encrypt %s: %w", field.scope, err)
		}
		*field.ptr = value
	}
	return st, nil
}

func decryptSettingsSecrets(st Settings) (Settings, error) {
	fields := []struct {
		scope string
		ptr   *string
	}{
		{"settings.default_api_key", &st.DefaultAPIKey},
		{"settings.whatsapp_token", &st.WhatsAppToken},
		{"settings.whatsapp_admin_token", &st.WhatsAppAdminToken},
		{"settings.stt_api_key", &st.STTAPIKey},
	}
	for _, field := range fields {
		value, err := securesecret.Decrypt(field.scope, *field.ptr)
		if err != nil {
			return Settings{}, fmt.Errorf("decrypt %s: %w", field.scope, err)
		}
		*field.ptr = value
	}
	return st, nil
}

func encryptProviderSecrets(provider Provider) (Provider, error) {
	value, err := securesecret.Encrypt("providers.api_key", provider.APIKey)
	if err != nil {
		return Provider{}, err
	}
	provider.APIKey = value
	return provider, nil
}

func decryptProviderSecrets(provider Provider) (Provider, error) {
	value, err := securesecret.Decrypt("providers.api_key", provider.APIKey)
	if err != nil {
		return Provider{}, err
	}
	provider.APIKey = value
	return provider, nil
}

func encryptChannelSecrets(channel Channel) (Channel, []byte, error) {
	plain := strings.TrimSpace(channel.Token)
	var hash []byte
	if plain != "" {
		if securesecret.IsEncrypted(plain) {
			decrypted, err := securesecret.Decrypt("channels.token", plain)
			if err != nil {
				return Channel{}, nil, err
			}
			plain = decrypted
		}
		hash = securesecret.LookupHash(plain)
	}
	value, err := securesecret.Encrypt("channels.token", plain)
	if err != nil {
		return Channel{}, nil, err
	}
	channel.Token = value
	return channel, hash, nil
}

func decryptChannelSecrets(channel Channel) (Channel, error) {
	value, err := securesecret.Decrypt("channels.token", channel.Token)
	if err != nil {
		return Channel{}, err
	}
	channel.Token = value
	return channel, nil
}

func integrationSecretHeader(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "authorization", "proxy-authorization", "x-api-key", "api-key", "apikey", "x-auth-token", "x-access-token", "cookie", "set-cookie":
		return true
	default:
		return strings.Contains(strings.ToLower(name), "secret") || strings.Contains(strings.ToLower(name), "token") || strings.Contains(strings.ToLower(name), "key")
	}
}

func transformIntegrationHeaders(raw string, decrypt bool) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return raw, nil
	}
	var headers map[string]string
	if err := json.Unmarshal([]byte(raw), &headers); err != nil {
		return "", err
	}
	for name, value := range headers {
		if !integrationSecretHeader(name) || strings.TrimSpace(value) == "" {
			continue
		}
		var err error
		scope := "integrations.headers." + strings.ToLower(strings.TrimSpace(name))
		if decrypt {
			headers[name], err = securesecret.Decrypt(scope, value)
		} else {
			headers[name], err = securesecret.Encrypt(scope, value)
		}
		if err != nil {
			return "", err
		}
	}
	encoded, err := json.Marshal(headers)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func encryptIntegrationSecrets(integration Integration) (Integration, error) {
	value, err := transformIntegrationHeaders(integration.Headers, false)
	if err != nil {
		return Integration{}, err
	}
	integration.Headers = value
	return integration, nil
}

func decryptIntegrationSecrets(integration Integration) (Integration, error) {
	value, err := transformIntegrationHeaders(integration.Headers, true)
	if err != nil {
		return Integration{}, err
	}
	integration.Headers = value
	return integration, nil
}
