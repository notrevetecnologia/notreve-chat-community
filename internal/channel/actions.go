package channel

import "strings"

type ConnectionState struct {
	Connected   bool
	LoggedIn    bool
	Ready       bool
	WebhookSet  bool
	Provisioned bool
}

func (s ConnectionState) IsConnected() bool {
	return s.Connected && s.LoggedIn
}

type Action string

const (
	ActionConnect    Action = "connect"
	ActionDisconnect Action = "disconnect"
	ActionLogout     Action = "logout"
	ActionTest       Action = "test"
)

func ParseAction(value string) (Action, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "connect", "conectar":
		return ActionConnect, true
	case "disconnect", "desconectar":
		return ActionDisconnect, true
	case "logout", "sair":
		return ActionLogout, true
	case "test", "testar", "status":
		return ActionTest, true
	}
	return "", false
}

func ActionAllowed(channelType string, action Action, state ConnectionState) (bool, string) {
	switch strings.ToLower(strings.TrimSpace(channelType)) {
	case "whatsapp":
		switch action {
		case ActionConnect:
			if state.IsConnected() {
				return false, "o canal já está conectado"
			}
			return true, ""
		case ActionDisconnect, ActionLogout:
			if !state.Connected {
				return false, "o canal já está desconectado"
			}
			return true, ""
		case ActionTest:
			if !state.IsConnected() {
				return false, "conecte o canal antes de testar"
			}
			return true, ""
		}
	case "telegram":
		configured := state.Ready || state.WebhookSet
		switch action {
		case ActionConnect:
			if configured {
				return false, "o webhook do Telegram já está configurado"
			}
			return true, ""
		case ActionDisconnect, ActionLogout:
			if !configured {
				return false, "o webhook do Telegram já está desativado"
			}
			return true, ""
		case ActionTest:
			return true, ""
		}
	case "webchat":
		if action == ActionTest {
			return true, ""
		}
		return false, "ação não disponível para o webchat"
	}
	return false, "ação não disponível para este canal"
}
