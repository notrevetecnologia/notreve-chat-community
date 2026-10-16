package store

import (
	"context"

	"github.com/notrevetecnologia/notreve-chat-community/internal/securesecret"
)

func (s *Store) GetSettingsSecure(ctx context.Context) (Settings, error) {
	settings, err := s.GetSettings(ctx)
	if err != nil {
		return Settings{}, err
	}
	return decryptSettingsSecrets(settings)
}

func (s *Store) UpdateSettingsSecure(ctx context.Context, settings Settings) error {
	stored, err := encryptSettingsSecrets(settings)
	if err != nil {
		return err
	}
	return s.UpdateSettings(ctx, stored)
}

func (s *Store) ListProvidersSecure(ctx context.Context) ([]Provider, error) {
	providers, err := s.ListProviders(ctx)
	if err != nil {
		return nil, err
	}
	for i := range providers {
		providers[i], err = decryptProviderSecrets(providers[i])
		if err != nil {
			return nil, err
		}
	}
	return providers, nil
}

func (s *Store) GetProviderSecure(ctx context.Context, id int) (Provider, error) {
	provider, err := s.GetProvider(ctx, id)
	if err != nil {
		return Provider{}, err
	}
	return decryptProviderSecrets(provider)
}

func (s *Store) FirstActiveProviderSecure(ctx context.Context) (Provider, error) {
	provider, err := s.FirstActiveProvider(ctx)
	if err != nil {
		return Provider{}, err
	}
	return decryptProviderSecrets(provider)
}

func (s *Store) CreateProviderSecure(ctx context.Context, provider Provider) (int, error) {
	stored, err := encryptProviderSecrets(provider)
	if err != nil {
		return 0, err
	}
	return s.CreateProvider(ctx, stored)
}

func (s *Store) UpdateProviderSecure(ctx context.Context, provider Provider) error {
	stored, err := encryptProviderSecrets(provider)
	if err != nil {
		return err
	}
	return s.UpdateProvider(ctx, stored)
}

func (s *Store) ListChannelsSecure(ctx context.Context) ([]Channel, error) {
	channels, err := s.ListChannels(ctx)
	if err != nil {
		return nil, err
	}
	for i := range channels {
		channels[i], err = decryptChannelSecrets(channels[i])
		if err != nil {
			return nil, err
		}
	}
	return channels, nil
}

func (s *Store) GetChannelByIDSecure(ctx context.Context, id int) (Channel, error) {
	channel, err := s.GetChannelByID(ctx, id)
	if err != nil {
		return Channel{}, err
	}
	return decryptChannelSecrets(channel)
}

func (s *Store) ActiveChannelSecure(ctx context.Context, channelType string) (Channel, error) {
	channel, err := s.ActiveChannel(ctx, channelType)
	if err != nil {
		return Channel{}, err
	}
	return decryptChannelSecrets(channel)
}

func (s *Store) CreateChannelSecure(ctx context.Context, channel Channel) (int, error) {
	stored, hash, err := encryptChannelSecrets(channel)
	if err != nil {
		return 0, err
	}
	id, err := s.CreateChannel(ctx, stored)
	if err != nil {
		return 0, err
	}
	if _, err := s.pool.Exec(ctx, `UPDATE channels SET token_hash=$1 WHERE tenant_id=$2 AND id=$3`, hash, TenantID(ctx), id); err != nil {
		_ = s.DeleteChannel(ctx, id)
		return 0, err
	}
	return id, nil
}

func (s *Store) UpdateChannelSecure(ctx context.Context, channel Channel) error {
	stored, hash, err := encryptChannelSecrets(channel)
	if err != nil {
		return err
	}
	if err := s.UpdateChannel(ctx, stored); err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `UPDATE channels SET token_hash=$1 WHERE tenant_id=$2 AND id=$3`, hash, TenantID(ctx), channel.ID)
	return err
}

func (s *Store) ListMonitorableChannelsSecure(ctx context.Context) ([]Channel, error) {
	channels, err := s.ListMonitorableChannels(ctx)
	if err != nil {
		return nil, err
	}
	for i := range channels {
		channels[i], err = decryptChannelSecrets(channels[i])
		if err != nil {
			return nil, err
		}
	}
	return channels, nil
}

func (s *Store) ListIntegrationsSecure(ctx context.Context) ([]Integration, error) {
	items, err := s.ListIntegrations(ctx)
	if err != nil {
		return nil, err
	}
	for i := range items {
		items[i], err = decryptIntegrationSecrets(items[i])
		if err != nil {
			return nil, err
		}
	}
	return items, nil
}

func (s *Store) GetIntegrationsByIDsSecure(ctx context.Context, ids []int) ([]Integration, error) {
	items, err := s.GetIntegrationsByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	for i := range items {
		items[i], err = decryptIntegrationSecrets(items[i])
		if err != nil {
			return nil, err
		}
	}
	return items, nil
}

func (s *Store) GetIntegrationSecure(ctx context.Context, id int) (Integration, error) {
	item, err := s.GetIntegration(ctx, id)
	if err != nil {
		return Integration{}, err
	}
	return decryptIntegrationSecrets(item)
}

func (s *Store) CreateIntegrationSecure(ctx context.Context, integration Integration) (int, error) {
	stored, err := encryptIntegrationSecrets(integration)
	if err != nil {
		return 0, err
	}
	return s.CreateIntegration(ctx, stored)
}

func (s *Store) UpdateIntegrationSecure(ctx context.Context, integration Integration) error {
	stored, err := encryptIntegrationSecrets(integration)
	if err != nil {
		return err
	}
	return s.UpdateIntegration(ctx, stored)
}

func (s *Store) CreateWhatsAppServerSecure(ctx context.Context, input WhatsAppServerInput) (WhatsAppServer, error) {
	stored, err := securesecret.Encrypt("whatsapp_servers.token", input.Token)
	if err != nil {
		return WhatsAppServer{}, err
	}
	input.Token = stored
	return s.CreateWhatsAppServer(ctx, input)
}

func (s *Store) UpdateWhatsAppServerSecure(ctx context.Context, serverID int64, input WhatsAppServerInput) error {
	if input.Token != "" {
		stored, err := securesecret.Encrypt("whatsapp_servers.token", input.Token)
		if err != nil {
			return err
		}
		input.Token = stored
	}
	return s.UpdateWhatsAppServer(ctx, serverID, input)
}

func (s *Store) WhatsAppServerCredentialsForChannelSecure(ctx context.Context, tenantID, channelID int) (WhatsAppServerCredentials, error) {
	credentials, err := s.WhatsAppServerCredentialsForChannel(ctx, tenantID, channelID)
	if err != nil {
		return WhatsAppServerCredentials{}, err
	}
	credentials.Token = ""
	return credentials, nil
}

func (s *Store) WhatsAppServerAdminCredentialsForChannelSecure(ctx context.Context, tenantID, channelID int) (WhatsAppServerCredentials, error) {
	credentials, err := s.WhatsAppServerCredentialsForChannel(ctx, tenantID, channelID)
	if err != nil {
		return WhatsAppServerCredentials{}, err
	}
	credentials.Token, err = securesecret.Decrypt("whatsapp_servers.token", credentials.Token)
	if err != nil {
		return WhatsAppServerCredentials{}, err
	}
	return credentials, nil
}
