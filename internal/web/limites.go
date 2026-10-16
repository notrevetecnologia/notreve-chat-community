package web

import (
	"context"

	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
)

// limiteCheck valida criação contra o plano. Zero significa ilimitado.
func (s *Server) limiteCheck(ctx context.Context, user *store.Usuario, uso, max int) bool {
	if user == nil || user.Role == "super_admin" || max == 0 {
		return true
	}
	return uso < max
}
