package web

import (
	"errors"
	"strings"
)

func campaignTransitionAllowed(from, action string) error {
	from = strings.ToLower(strings.TrimSpace(from))
	action = strings.ToLower(strings.TrimSpace(action))
	switch action {
	case "iniciar", "retomar":
		switch from {
		case "rascunho", "agendada", "pausada":
			return nil
		case "rodando":
			return errors.New("a campanha já está em andamento")
		case "concluida":
			return errors.New("campanhas concluídas não podem ser reiniciadas")
		case "cancelada":
			return errors.New("campanhas canceladas não podem ser reiniciadas")
		}
	case "pausar":
		switch from {
		case "rodando":
			return nil
		case "pausada":
			return errors.New("a campanha já está pausada")
		default:
			return errors.New("só é possível pausar uma campanha em andamento")
		}
	case "cancelar":
		switch from {
		case "rascunho", "agendada", "rodando", "pausada":
			return nil
		case "concluida":
			return errors.New("campanhas concluídas não podem ser canceladas")
		case "cancelada":
			return errors.New("a campanha já está cancelada")
		}
	}
	return errors.New("ação de campanha inválida")
}

func campaignDeleteAllowed(status string) error {
	if strings.ToLower(strings.TrimSpace(status)) == "rodando" {
		return errors.New("cancele a campanha antes de excluí-la")
	}
	return nil
}
