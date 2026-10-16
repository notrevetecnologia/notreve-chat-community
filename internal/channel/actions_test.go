package channel

import "testing"

func TestParseActionAcceptsLegacyAliases(t *testing.T) {
	cases := map[string]Action{
		"connect": ActionConnect, "conectar": ActionConnect,
		"disconnect": ActionDisconnect, "desconectar": ActionDisconnect,
		"logout": ActionLogout, "sair": ActionLogout,
		"test": ActionTest, "testar": ActionTest, "status": ActionTest,
	}
	for input, want := range cases {
		got, ok := ParseAction(input)
		if !ok || got != want {
			t.Fatalf("ParseAction(%q) = %q,%v; esperado %q,true", input, got, ok, want)
		}
	}
	if _, ok := ParseAction("excluir"); ok {
		t.Fatal("ação desconhecida não deveria ser aceita")
	}
}

func TestWhatsAppActionAllowedByState(t *testing.T) {
	disconnected := ConnectionState{Provisioned: true}
	connecting := ConnectionState{Connected: true, Provisioned: true}
	connected := ConnectionState{Connected: true, LoggedIn: true, Provisioned: true}

	for _, tc := range []struct {
		name    string
		state   ConnectionState
		action  Action
		allowed bool
	}{
		{"conectar desconectado", disconnected, ActionConnect, true},
		{"conectar conectando", connecting, ActionConnect, true},
		{"conectar conectado", connected, ActionConnect, false},
		{"desconectar desconectado", disconnected, ActionDisconnect, false},
		{"desconectar conectando", connecting, ActionDisconnect, true},
		{"desconectar conectado", connected, ActionDisconnect, true},
		{"logout desconectado", disconnected, ActionLogout, false},
		{"logout conectado", connected, ActionLogout, true},
		{"testar desconectado", disconnected, ActionTest, false},
		{"testar conectando", connecting, ActionTest, false},
		{"testar conectado", connected, ActionTest, true},
	} {
		allowed, message := ActionAllowed("whatsapp", tc.action, tc.state)
		if allowed != tc.allowed {
			t.Fatalf("%s: allowed=%v (%q), esperado %v", tc.name, allowed, message, tc.allowed)
		}
		if !allowed && message == "" {
			t.Fatalf("%s: ação inválida precisa de motivo", tc.name)
		}
	}
}

func TestTelegramActionAllowedByState(t *testing.T) {
	pending := ConnectionState{Provisioned: true}
	ready := ConnectionState{Ready: true, WebhookSet: true, Provisioned: true}

	for _, tc := range []struct {
		name    string
		state   ConnectionState
		action  Action
		allowed bool
	}{
		{"conectar pendente", pending, ActionConnect, true},
		{"conectar pronto", ready, ActionConnect, false},
		{"desconectar pendente", pending, ActionDisconnect, false},
		{"desconectar pronto", ready, ActionDisconnect, true},
		{"testar pendente", pending, ActionTest, true},
		{"testar pronto", ready, ActionTest, true},
	} {
		allowed, message := ActionAllowed("telegram", tc.action, tc.state)
		if allowed != tc.allowed {
			t.Fatalf("%s: allowed=%v (%q), esperado %v", tc.name, allowed, message, tc.allowed)
		}
	}
}

func TestWebchatOnlyAllowsTest(t *testing.T) {
	state := ConnectionState{Ready: true}
	if allowed, _ := ActionAllowed("webchat", ActionTest, state); !allowed {
		t.Fatal("testar deveria ser permitido no webchat")
	}
	for _, action := range []Action{ActionConnect, ActionDisconnect, ActionLogout} {
		if allowed, _ := ActionAllowed("webchat", action, state); allowed {
			t.Fatalf("ação %q não deveria ser permitida no webchat", action)
		}
	}
}
