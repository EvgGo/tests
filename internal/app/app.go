package app

import (
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"testing/internal/config"
	"testing/internal/grpcapp"
	"testing/internal/grpcapp/jwtverify"
	postgresDB "testing/internal/infrastracture/db/postgres"
	repopostgres "testing/internal/repo/postgres"
	assessmentsvc "testing/internal/service/assessment"
	transportgrpc "testing/internal/transport/grpc"
)

type App struct {
	GRPCSrv *grpcapp.App
	db      *pgxpool.Pool
}

// New создает Assessment приложение:
// - поднимает pgxpool
// - собирает repositories
// - собирает tx manager
// - собирает service layer
// - собирает JWT verifier
// - собирает gRPC server AdaptiveTesting
// - собирает grpcapp
func New(
	log *slog.Logger,
	cfg *config.Config,
) (*App, error) {

	pool, err := postgresDB.NewPool(&cfg.Postgres)
	if err != nil {
		return nil, fmt.Errorf("init postgres: %w", err)
	}

	cleanupOnErr := func(e error) (*App, error) {
		pool.Close()
		return nil, e
	}

	txManager := repopostgres.NewTxManager(pool)
	catalogRepo := repopostgres.NewCatalogRepo(pool)
	questionRepo := repopostgres.NewQuestionRepo(pool)
	attemptRepo := repopostgres.NewAttemptRepo(pool)

	service := assessmentsvc.New(
		txManager,
		catalogRepo,
		questionRepo,
		attemptRepo,
	)
	if service == nil {
		return cleanupOnErr(fmt.Errorf("init assessment service: nil"))
	}

	verifier, err := jwtverify.NewHSVerifier(jwtverify.HSOptions{
		Issuer:     cfg.Auth.JWT.Issuer,
		SigningKey: []byte(cfg.Auth.JWT.SigningKey),
		ClockSkew:  cfg.Auth.JWT.ClockSkew,
	})
	if err != nil {
		return cleanupOnErr(fmt.Errorf("init jwt verifier: %w", err))
	}

	adaptiveTestingServer := transportgrpc.NewServer(log, service)
	if adaptiveTestingServer == nil {
		return cleanupOnErr(fmt.Errorf("init adaptive testing grpc server: nil"))
	}

	grpcApp := grpcapp.New(log, cfg.GRPCAddr.Port, grpcapp.Deps{
		AdaptiveTesting: adaptiveTestingServer,
		JWT:             verifier,
		Timeout:         cfg.GRPCAddr.Timeout,
	})
	if grpcApp == nil {
		return cleanupOnErr(fmt.Errorf("init grpc app: nil"))
	}

	return &App{
		GRPCSrv: grpcApp,
		db:      pool,
	}, nil
}

func (a *App) Close() {
	if a.db != nil {
		a.db.Close()
	}
}
