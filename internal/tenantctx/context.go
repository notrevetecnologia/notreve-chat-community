package tenantctx

import "context"

type tenantKey struct{}
type bypassKey struct{}

func WithID(ctx context.Context, tenantID int) context.Context {
	return context.WithValue(ctx, tenantKey{}, tenantID)
}

func ID(ctx context.Context) (int, bool) {
	if ctx == nil {
		return 0, false
	}
	tenantID, ok := ctx.Value(tenantKey{}).(int)
	return tenantID, ok
}

// WithRLSBypass marca operações globais deliberadas (control plane,
// autenticação e roteamento público). O bypass nunca é inferido pela ausência
// de tenant: precisa ser solicitado explicitamente pelo código.
func WithRLSBypass(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, bypassKey{}, true)
}

func RLSBypass(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	value, _ := ctx.Value(bypassKey{}).(bool)
	return value
}
