package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/KubeRocketCI/gitfusion/internal/api"
	buildInfo "github.com/epam/edp-common/pkg/config"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httplog/v2"
)

func main() {
	config := api.GetConfigOrDie()
	logger := initLogger(config)
	info := buildInfo.Get()

	logger.Info("Starting the GitFusion API server",
		"version", info.Version,
		"git-commit", info.GitCommit,
		"git-tag", info.GitTag,
		"build-date", info.BuildDate,
		"go-version", info.Go,
		"go-client", info.KubectlVersion,
		"platform", info.Platform,
	)

	r := chi.NewMux()
	r.Use(middleware.RequestID)
	r.Use(httplog.RequestLogger(logger))
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))
	r.Use(middleware.Heartbeat("/healthz"))

	handler, err := api.BuildHandler(config)
	if err != nil {
		log.Fatal(err)
	}

	server := &http.Server{
		Handler: api.HandlerFromMux(handler, r),
		Addr:    ":" + config.Port,
	}

	// Server run context
	serverCtx, serverStopCtx := context.WithCancel(context.Background())

	// Listen for syscall signals for process to interrupt/quit
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	go func() {
		<-sig

		// Shutdown signal with grace period of 30 seconds
		shutdownCtx, cancel := context.WithTimeout(serverCtx, 30*time.Second)
		defer cancel()

		go func() {
			<-shutdownCtx.Done()

			if shutdownCtx.Err() == context.DeadlineExceeded {
				log.Fatal("graceful shutdown timed out.. forcing exit.")
			}
		}()

		// Trigger graceful shutdown
		err := server.Shutdown(shutdownCtx)
		if err != nil {
			log.Fatal(err)
		}

		serverStopCtx()
	}()

	// Run the server
	if err = server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}

	// Wait for server context to be stopped
	<-serverCtx.Done()
}

func initLogger(config api.Config) *httplog.Logger {
	return httplog.NewLogger("gitfusion-api", httplog.Options{
		JSON:            true,
		LogLevel:        slog.LevelDebug,
		Concise:         true,
		RequestHeaders:  false,
		TimeFieldFormat: time.RFC3339,
		Tags: map[string]string{
			"ns": config.Namespace,
		},
		QuietDownRoutes: []string{
			"/",
			"/health",
		},
		QuietDownPeriod: 10 * time.Second,
		SourceFieldName: "source",
	})
}
