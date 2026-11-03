package main

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/rohithsandiri/spendsense-backend/internal/config"
	"github.com/rohithsandiri/spendsense-backend/internal/db"
	"github.com/rohithsandiri/spendsense-backend/internal/logger"
	"github.com/rohithsandiri/spendsense-backend/internal/router"
)

func main() {
	cfg := config.Load()
	logger.Init(cfg.LogstashHost, cfg.LogstashPort)

	ctx := context.Background()
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("PostgreSQL connection failed: " + err.Error())
		os.Exit(1)
	}
	defer pool.Close()
	logger.Info("PostgreSQL connected successfully")

	handler := router.New(pool, cfg.JWTSecret)
	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	logger.Info("Server running on port " + cfg.Port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("Server error: " + err.Error())
		os.Exit(1)
	}
}
