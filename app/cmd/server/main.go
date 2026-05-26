package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib" // goose用のdatabase/sqlドライバー
	"github.com/line/line-bot-sdk-go/v8/linebot"
	"github.com/pressly/goose/v3"

	"github.com/kkitai/line-claude-observer/app/internal/config"
	"github.com/kkitai/line-claude-observer/app/internal/handler"
	"github.com/kkitai/line-claude-observer/app/internal/line"
	"github.com/kkitai/line-claude-observer/app/internal/repository"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// DB接続
	db, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(context.Background()); err != nil {
		log.Fatalf("failed to ping database: %v", err)
	}

	// gooseマイグレーション実行
	if err := runMigrations(cfg.DatabaseURL); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}

	// LINE botクライアント（署名検証用）
	bot, err := linebot.New(cfg.LineChannelSecret, cfg.LineChannelAccessToken)
	if err != nil {
		log.Fatalf("failed to create linebot client: %v", err)
	}

	// LINE APIクライアント（プロフィール取得・返信用）
	lineClient, err := line.New(cfg.LineChannelSecret, cfg.LineChannelAccessToken)
	if err != nil {
		log.Fatalf("failed to create line client: %v", err)
	}

	// Repository
	groupRepo := repository.NewGroupRepository(db)
	messageRepo := repository.NewMessageRepository(db)

	// Handler
	webhookHandler := handler.NewWebhookHandler(bot, groupRepo, messageRepo, lineClient, cfg.OpinionCommand)

	// Router
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Post("/webhook", webhookHandler.ServeHTTP)
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintln(w, "ok")
	})

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 40 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// グレースフルシャットダウン
	go func() {
		log.Printf("starting server on port %s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("server forced to shutdown: %v", err)
	}
	log.Println("server stopped")
}

func runMigrations(databaseURL string) error {
	db, err := goose.OpenDBWithDriver("pgx", databaseURL)
	if err != nil {
		return fmt.Errorf("goose open db: %w", err)
	}
	defer func() { _ = db.Close() }()

	migrationsDir := "app/db/migrations"
	if err := goose.Up(db, migrationsDir); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}
	return nil
}
