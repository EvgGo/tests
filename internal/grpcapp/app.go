package grpcapp

import (
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"sync/atomic"
	"time"

	assessmentv1 "github.com/EvgGo/proto/proto/gen/go/tests"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	grpc_health_v1 "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	"testing/internal/grpcapp/interceptors"
	"testing/internal/grpcapp/jwtverify"
)

type Deps struct {
	AdaptiveTesting assessmentv1.AdaptiveTestingServer

	JWT     jwtverify.Verifier
	Timeout time.Duration

	// allowUnauthenticated - список методов, которые можно вызывать без токена
	AllowUnauthenticated map[string]bool
}

type App struct {
	log  *slog.Logger
	port int
	srv  *grpc.Server
	lis  net.Listener

	started atomic.Bool
}

// New создает gRPC сервер, цепляет интерцепторы и регистрирует сервисы
func New(log *slog.Logger, port int, deps Deps) *App {
	if log == nil {
		log = slog.Default()
	}

	if deps.Timeout <= 0 {
		deps.Timeout = 10 * time.Second
	}

	if deps.AllowUnauthenticated == nil {
		deps.AllowUnauthenticated = defaultAllowUnauth()
	}

	if deps.AdaptiveTesting == nil {
		panic("grpcapp.New: AdaptiveTesting server is nil")
	}
	if deps.JWT == nil {
		panic("grpcapp.New: JWT verifier is nil")
	}

	s := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			interceptors.RequestIDUnaryInterceptor(),
			interceptors.RecovererUnaryInterceptor(log),
			interceptors.TimeoutUnaryInterceptor(deps.Timeout),
			interceptors.AuthUnaryInterceptor(log, deps.JWT, deps.AllowUnauthenticated),
			interceptors.LoggingUnaryInterceptor(log),
		),
		grpc.ChainStreamInterceptor(
			interceptors.RequestIDStreamInterceptor(),
			interceptors.RecovererStreamInterceptor(log),
			interceptors.AuthStreamInterceptor(log, deps.JWT, deps.AllowUnauthenticated),
			interceptors.LoggingStreamInterceptor(log),
		),
	)

	assessmentv1.RegisterAdaptiveTestingServer(s, deps.AdaptiveTesting)

	hs := health.NewServer()
	grpc_health_v1.RegisterHealthServer(s, hs)
	hs.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)

	reflection.Register(s)

	log.Info("gRPC server initialized",
		"port", port,
		"adaptive_testing_registered", true,
		"timeout", deps.Timeout,
	)

	return &App{
		log:  log,
		port: port,
		srv:  s,
	}
}

// MustRun поднимает listener и начинает Serve
func (a *App) MustRun() {
	addr := ":" + strconv.Itoa(a.port)

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		panic(fmt.Errorf("grpc listen %s: %w", addr, err))
	}

	a.lis = lis
	a.started.Store(true)

	a.log.Info("gRPC server started", "addr", addr)

	if err = a.srv.Serve(lis); err != nil {
		panic(fmt.Errorf("grpc serve: %w", err))
	}
}

// StopGracefully пытается корректно завершить работу
func (a *App) StopGracefully() {
	if !a.started.Load() {
		return
	}

	a.log.Info("gRPC server stopping (graceful)")
	a.srv.GracefulStop()

	if a.lis != nil {
		_ = a.lis.Close()
	}

	a.started.Store(false)
}

// StopNow - жесткая остановка
func (a *App) StopNow() {
	if !a.started.Load() {
		return
	}

	a.log.Warn("gRPC server stopping (force)")
	a.srv.Stop()

	if a.lis != nil {
		_ = a.lis.Close()
	}

	a.started.Store(false)
}

func defaultAllowUnauth() map[string]bool {
	return map[string]bool{
		// health:
		"/grpc.health.v1.Health/Check": true,
		"/grpc.health.v1.Health/Watch": true,

		// reflection:
		"/grpc.reflection.v1alpha.ServerReflection/ServerReflectionInfo": true,
	}
}
