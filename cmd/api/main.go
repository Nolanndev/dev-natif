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

	// --- Use-case services --------------------------------------------------
	projectSvc := service.NewProjectService(st)
	deploymentSvc := service.NewDeploymentService(st, st, st, engine)

	// --- HTTP layer ---------------------------------------------------------
	router := httpapi.NewRouter(httpapi.Deps{
		Logger:      logger,
		APIKey:      cfg.APIKey,
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
