// Command api is the entrypoint of the dev-natif native Docker API.
//
// It wires the adapters (SQLite store, Docker engine) to the use-case services
// and the Gin HTTP layer, then serves until interrupted. This file is the
// single composition root: it is the authoritative contract for every
// constructor signature the adapter packages must provide.
package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Nolanndev/dev-natif/internal/auth"
	"github.com/Nolanndev/dev-natif/internal/config"
	dockeng "github.com/Nolanndev/dev-natif/internal/docker"
	httpapi "github.com/Nolanndev/dev-natif/internal/http"
	"github.com/Nolanndev/dev-natif/internal/logging"
	"github.com/Nolanndev/dev-natif/internal/service"
	"github.com/Nolanndev/dev-natif/internal/store"
)

func main() {
	cfg := config.Load()
	logger := logging.New(cfg.LogLevel)

	// --- Adapters -----------------------------------------------------------
	st, err := store.New(cfg.DBPath)
	if err != nil {
		logger.Error("failed to open store", "error", err)
		os.Exit(1)
	}
	defer st.Close()

	engine, err := dockeng.New(cfg.DockerHost)
	if err != nil {
		logger.Error("failed to init docker engine", "error", err)
		os.Exit(1)
	}

	// Best-effort connectivity check (non-fatal: API still serves CRUD).
	pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if perr := engine.Ping(pingCtx); perr != nil {
		logger.Warn("docker engine unreachable at startup", "error", perr)
	}
	cancel()

	// --- Authentication -----------------------------------------------------
	authn := auth.New(auth.Config{
		Enabled:  cfg.AuthEnabled,
		Username: cfg.AuthUsername,
		Password: cfg.AuthPassword,
		Secret:   cfg.JWTSecret,
		TTL:      cfg.TokenTTL,
	})
	if cfg.AuthEnabled {
		if cfg.AuthPassword == "admin" {
			logger.Warn("auth enabled with the default password 'admin' — set NATIF_AUTH_PASSWORD")
		}
		if authn.UsesGeneratedSecret() {
			logger.Warn("NATIF_JWT_SECRET not set — using a random secret; tokens won't survive a restart")
		}
		logger.Info("api authentication enabled", "token_ttl", cfg.TokenTTL.String())
	} else {
		logger.Warn("api authentication is DISABLED (set NATIF_AUTH_ENABLED=true to enforce)")
	}

	// --- Use-case services --------------------------------------------------
	projectSvc := service.NewProjectService(st)
	deploymentSvc := service.NewDeploymentService(st, st, st, engine, st)

	// --- Retention: purge events/history older than the window --------------
	if cfg.RetentionDays > 0 {
		retention := time.Duration(cfg.RetentionDays) * 24 * time.Hour
		go func() {
			purge := func() {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				if n, err := st.PurgeEventsBefore(ctx, time.Now().Add(-retention)); err != nil {
					logger.Warn("event purge failed", "error", err)
				} else if n > 0 {
					logger.Info("purged old events", "count", n, "older_than_days", cfg.RetentionDays)
				}
			}
			purge() // once at startup
			t := time.NewTicker(6 * time.Hour)
			defer t.Stop()
			for range t.C {
				purge()
			}
		}()
		logger.Info("event retention enabled", "days", cfg.RetentionDays)
	}

	// --- HTTP layer ---------------------------------------------------------
	router := httpapi.NewRouter(httpapi.Deps{
		Logger:      logger,
		Auth:        authn,
		Projects:    projectSvc,
		Deployments: deploymentSvc,
		Servers:     st,
		Engine:      engine,
	})

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// --- Graceful shutdown --------------------------------------------------
	go func() {
		logger.Info("api listening", "port", cfg.Port, "db", cfg.DBPath)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	logger.Info("shutting down")
	shutCtx, shutCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutCancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
	}
}
