package interceptors

import (
	"context"
	"github.com/google/uuid"
	"testing/internal/authctx"
	"testing/internal/grpcapp/jwtverify"

	"log/slog"
	"runtime/debug"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

// RequestIDUnaryInterceptor:
// - читает x-request-id из incoming metadata (если есть)
// - иначе генерит UUID
// - кладет в ctx (authctx.WithRequestID)
// - возвращает x-request-id в response headers
func RequestIDUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		rid := requestIDFromIncoming(ctx)
		ctx = authctx.WithRequestID(ctx, rid)

		// Отдадим request id назад клиенту
		_ = grpc.SetHeader(ctx, metadata.Pairs("x-request-id", rid))

		return handler(ctx, req)
	}
}

func RequestIDStreamInterceptor() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		rid := requestIDFromIncoming(ss.Context())
		ctx := authctx.WithRequestID(ss.Context(), rid)

		_ = ss.SetHeader(metadata.Pairs("x-request-id", rid))

		return handler(srv, &wrappedServerStream{ServerStream: ss, ctx: ctx})
	}
}

func requestIDFromIncoming(ctx context.Context) string {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if vals := md.Get("x-request-id"); len(vals) > 0 {
			rid := strings.TrimSpace(vals[0])
			if rid != "" {
				return rid
			}
		}
	}
	// fallback: генерим UUID
	u, err := uuid.NewV7()
	if err != nil {
		// в крайне редком случае - time-based fallback
		return time.Now().UTC().Format(time.RFC3339Nano)
	}
	return u.String()
}

// TimeoutUnaryInterceptor ставит deadline для unary RPC, если его не поставил клиент/gateway
func TimeoutUnaryInterceptor(defaultTimeout time.Duration) grpc.UnaryServerInterceptor {
	// если timeout не задан - не вмешиваемся
	if defaultTimeout <= 0 {
		return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
			return handler(ctx, req)
		}
	}

	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if _, ok := ctx.Deadline(); ok {
			return handler(ctx, req)
		}
		ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
		defer cancel()
		return handler(ctx, req)
	}
}

// RecovererUnaryInterceptor перехватывает panic и возвращает codes.Internal
func RecovererUnaryInterceptor(log *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		defer func() {
			if r := recover(); r != nil {
				rid, _ := authctx.RequestID(ctx)
				log.Error("panic recovered",
					"method", info.FullMethod,
					"request_id", rid,
					"panic", r,
					"stack", string(debug.Stack()),
				)
				err = status.Error(codes.Internal, "internal error")
			}
		}()
		return handler(ctx, req)
	}
}

func RecovererStreamInterceptor(log *slog.Logger) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
		defer func() {
			if r := recover(); r != nil {
				rid, _ := authctx.RequestID(ss.Context())
				log.Error("panic recovered",
					"method", info.FullMethod,
					"request_id", rid,
					"panic", r,
					"stack", string(debug.Stack()),
				)
				err = status.Error(codes.Internal, "internal error")
			}
		}()
		return handler(srv, ss)
	}
}

// AuthUnaryInterceptor валидирует access JWT из metadata:
//
//	authorization: Bearer token
//
// На успех: кладeт user_id в ctx.
// На неуспех: возвращает Unauthenticated и пишет warn лог
//
// allowUnauthenticated: карта методов, которые можно вызывать без токена
func AuthUnaryInterceptor(log *slog.Logger, v jwtverify.Verifier, allowUnauthenticated map[string]bool) grpc.UnaryServerInterceptor {
	if allowUnauthenticated == nil {
		allowUnauthenticated = map[string]bool{}
	}

	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if allowUnauthenticated[info.FullMethod] {
			return handler(ctx, req)
		}

		token := bearerTokenFromIncoming(ctx)
		if token == "" {
			logUnauth(log, ctx, info.FullMethod, "missing bearer token")
			return nil, status.Error(codes.Unauthenticated, "missing authorization token")
		}

		claims, err := v.VerifyAccess(ctx, token)
		if err != nil {
			// детали ошибки не отдаeм наружу, но логируем
			logUnauth(log, ctx, info.FullMethod, err.Error())
			return nil, status.Error(codes.Unauthenticated, "invalid authorization token")
		}
		if strings.TrimSpace(claims.UserID) == "" {
			logUnauth(log, ctx, info.FullMethod, "missing sub/user_id in claims")
			return nil, status.Error(codes.Unauthenticated, "invalid authorization token")
		}

		ctx = authctx.WithUserID(ctx, claims.UserID)
		return handler(ctx, req)
	}
}

func AuthStreamInterceptor(log *slog.Logger, v jwtverify.Verifier, allowUnauthenticated map[string]bool) grpc.StreamServerInterceptor {
	if allowUnauthenticated == nil {
		allowUnauthenticated = map[string]bool{}
	}

	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if allowUnauthenticated[info.FullMethod] {
			return handler(srv, ss)
		}

		ctx := ss.Context()
		token := bearerTokenFromIncoming(ctx)
		if token == "" {
			logUnauth(log, ctx, info.FullMethod, "missing bearer token")
			return status.Error(codes.Unauthenticated, "missing authorization token")
		}

		claims, err := v.VerifyAccess(ctx, token)
		if err != nil {
			logUnauth(log, ctx, info.FullMethod, err.Error())
			return status.Error(codes.Unauthenticated, "invalid authorization token")
		}
		if strings.TrimSpace(claims.UserID) == "" {
			logUnauth(log, ctx, info.FullMethod, "missing sub/user_id in claims")
			return status.Error(codes.Unauthenticated, "invalid authorization token")
		}

		newCtx := authctx.WithUserID(ctx, claims.UserID)
		return handler(srv, &wrappedServerStream{ServerStream: ss, ctx: newCtx})
	}
}

func logUnauth(log *slog.Logger, ctx context.Context, method, reason string) {
	rid, _ := authctx.RequestID(ctx)
	p := peerString(ctx)
	log.Warn("unauthenticated request",
		"method", method,
		"request_id", rid,
		"peer", p,
		"reason", reason,
	)
}

// bearerTokenFromIncoming достаeт token без Bearer
func bearerTokenFromIncoming(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	vals := md.Get("authorization")
	if len(vals) == 0 {
		return ""
	}
	h := strings.TrimSpace(vals[0])
	if h == "" {
		return ""
	}
	parts := strings.SplitN(h, " ", 2)
	if len(parts) != 2 {
		return ""
	}
	if !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	tok := strings.TrimSpace(parts[1])
	if tok == "" {
		return ""
	}
	return tok
}

// LoggingUnaryInterceptor логирует результат вызова (код/время)
// Ставим его ПОСЛЕ Auth, чтобы user_id уже был в ctx
func LoggingUnaryInterceptor(log *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		dur := time.Since(start)

		code := status.Code(err)
		level := levelByCode(code)

		rid, _ := authctx.RequestID(ctx)
		uid, _ := authctx.UserID(ctx)

		log.Log(ctx, level, "grpc request",
			"method", info.FullMethod,
			"code", code.String(),
			"duration_ms", dur.Milliseconds(),
			"request_id", rid,
			"user_id", uid,
			"peer", peerString(ctx),
		)

		return resp, err
	}
}

func LoggingStreamInterceptor(log *slog.Logger) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		start := time.Now()
		err := handler(srv, ss)
		dur := time.Since(start)

		code := status.Code(err)
		level := levelByCode(code)

		ctx := ss.Context()
		rid, _ := authctx.RequestID(ctx)
		uid, _ := authctx.UserID(ctx)

		log.Log(ctx, level, "grpc stream",
			"method", info.FullMethod,
			"code", code.String(),
			"duration_ms", dur.Milliseconds(),
			"request_id", rid,
			"user_id", uid,
			"peer", peerString(ctx),
		)

		return err
	}
}

func levelByCode(code codes.Code) slog.Level {
	switch code {
	case codes.OK:
		return slog.LevelInfo
	case codes.InvalidArgument, codes.NotFound, codes.AlreadyExists, codes.FailedPrecondition:
		return slog.LevelInfo
	case codes.Unauthenticated, codes.PermissionDenied, codes.ResourceExhausted:
		return slog.LevelWarn
	default:
		return slog.LevelError
	}
}

func peerString(ctx context.Context) string {
	if p, ok := peer.FromContext(ctx); ok && p.Addr != nil {
		return p.Addr.String()
	}
	return ""
}

// wrappedServerStream позволяет подменить context у stream,
// чтобы прокинуть request_id/user_id из интерцептора
type wrappedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedServerStream) Context() context.Context {
	return w.ctx
}
