package blathers

import "context"

func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, "request_id", requestID)
}

func RequestID(ctx context.Context) string {
	ret := ctx.Value("request_id")
	if ret == nil {
		return "no_provided_request_id"
	}
	if ret == "" {
		return "blank_request_id"
	}
	return ret.(string)
}

func WithDebuggingPrefix(ctx context.Context, text string) context.Context {
	t := text + ": "
	prev := ctx.Value("debugging_prefix")
	if prev != nil {
		t = prev.(string) + t
	}
	return context.WithValue(ctx, "debugging_prefix", t)
}

func DebuggingPrefix(ctx context.Context) string {
	ret := ctx.Value("debugging_prefix")
	if ret == nil {
		return ""
	}
	return ret.(string)
}
