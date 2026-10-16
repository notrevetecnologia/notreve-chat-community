package web

import (
	"errors"
	"strings"

	"github.com/notrevetecnologia/notreve-chat-community/internal/agendamento"
)

func scheduleStatusTransitionAllowed(from, to string) error {
	from = strings.ToLower(strings.TrimSpace(from))
	to = strings.ToLower(strings.TrimSpace(to))
	if from == to {
		return nil
	}
	switch to {
	case agendamento.StatusPendente:
		switch from {
		case agendamento.StatusCancelada, agendamento.StatusFalhou:
			return nil
		case agendamento.StatusConcluida:
			return errors.New("agendamentos concluídos não podem ser reativados")
		}
	case agendamento.StatusCancelada:
		switch from {
		case agendamento.StatusPendente, agendamento.StatusExecutando, agendamento.StatusFalhou:
			return nil
		case agendamento.StatusConcluida:
			return errors.New("agendamentos concluídos não podem ser cancelados")
		}
	}
	return errors.New("transição de status do agendamento não permitida")
}
