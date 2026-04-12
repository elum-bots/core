package bot

import "context"

type chainedHandlerKey struct{}

func withChainedNext(ctx context.Context) context.Context {
	return context.WithValue(ctx, chainedHandlerKey{}, true)
}

func HasChainedNext(ctx context.Context) bool {
	v := ctx.Value(chainedHandlerKey{})
	ok, _ := v.(bool)
	return ok
}
