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
	"github.com/KubeRocketCI/gitfusion/internal/services"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httplog/v2"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

func main() {
	cfg := config.GetConfigOrDie()

	_, _ = client.New(cfg, client.Options{})

	logger := httplog.NewLogger("httplog-example", httplog.Options{
		JSON:            true,
		LogLevel:        slog.LevelDebug,
		Concise:         true,
		RequestHeaders:  false,
		TimeFieldFormat: time.RFC3339,
		Tags: map[string]string{
			"ns": "dev", //TODO
		},
		QuietDownRoutes: []string{
			"/",
			"/health",
		},
		QuietDownPeriod: 10 * time.Second,
		SourceFieldName: "source",
	})

	r := chi.NewMux()
	r.Use(middleware.RequestID)
	r.Use(httplog.RequestLogger(logger))
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))
	r.Use(middleware.Heartbeat("/health"))

	handler := api.NewStrictHandlerWithOptions(
		api.NewServer(services.NewRepositoriesService(nil, nil)),
		[]api.StrictMiddlewareFunc{},
		api.StrictHTTPServerOptions{},
	)

	h := api.HandlerFromMux(handler, r)

	server := &http.Server{
		Handler: h,
		Addr:    ":8080",
	}

	// Server run context
	serverCtx, serverStopCtx := context.WithCancel(context.Background())

	// Listen for syscall signals for process to interrupt/quit
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
		<-sig

		// Shutdown signal with grace period of 30 seconds
		shutdownCtx, _ := context.WithTimeout(serverCtx, 30*time.Second)

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
	err := server.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}

	// Wait for server context to be stopped
	<-serverCtx.Done()
}
