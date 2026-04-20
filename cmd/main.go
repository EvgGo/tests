package main

import (
	"github.com/joho/godotenv"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"testing/internal/app"
	"testing/internal/config"
	"testing/pkg/logger"
)

func main() {

	if err := godotenv.Load(".env"); err != nil {
		log.Fatalf("Внимание: файл .env не найден, используются переменные окружения по умолчанию")
	}

	cfg := config.MustLoad("CONFIG_PATH_TESTS")
	l, err := logger.SetupLogger(cfg.Env, "", cfg.LogLevel, cfg.LogFile)
	if err != nil {
		log.Fatalf("Ошибка при инициализации логгера: %v", err)
	}

	application, err := app.New(l, cfg)
	if err != nil {
		l.Error("build app failed", "err", err)
		os.Exit(1)
	}

	l.Debug("application успешно создано")

	application.GRPCSrv.MustRun()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)

	sign := <-stop

	l.Info("stopping application", slog.String("signal", sign.String()))

	application.GRPCSrv.StopGracefully()

	l.Info("application stopped")
}
