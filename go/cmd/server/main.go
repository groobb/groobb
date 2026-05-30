// Command server is the entry point for the Groobb HTTP server.
// [Ja] server コマンドは Groobb HTTP サーバーのエントリポイントです。
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/handler/health"
	"github.com/groobb/groobb/go/internal/handler/welcome"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load the configuration", "error", err)
		os.Exit(1)
	}

	// Connect to PostgreSQL and verify connectivity before serving requests, so
	// a misconfigured or unreachable database fails fast at startup. The bounded
	// context only guards the initial connect/ping; the pool itself outlives it.
	//
	// [Ja] リクエストを受ける前に PostgreSQL へ接続して疎通を確認し、設定ミスや
	// 接続不能なデータベースを起動時に早期検知する。タイムアウト付き context は
	// 最初の接続/ping だけを制御し、プール自体はそれより長く生存する。
	connectCtx, connectCancel := context.WithTimeout(context.Background(), 10*time.Second)
	pool, err := database.New(connectCtx, cfg.DatabaseURL)
	connectCancel()
	if err != nil {
		slog.Error("failed to connect to the database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()
	slog.Info("connected to the database")

	healthHandler := health.NewHandler()
	welcomeHandler := welcome.NewHandler(cfg)

	r := chi.NewRouter()
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.Recoverer)

	// Health check (no authentication required).
	// [Ja] ヘルスチェック (認証不要)。
	r.Get("/health", healthHandler.Show)

	// Serve static assets (CSS / JS / images) built into ./static.
	// [Ja] ./static にビルドされた静的アセット (CSS / JS / 画像) を配信する。
	fileServer := http.FileServer(http.Dir("./static"))
	r.Handle("/static/*", http.StripPrefix("/static", fileServer))

	// Top page.
	// [Ja] トップページ。
	r.Get("/", welcomeHandler.Show)

	addr := fmt.Sprintf("0.0.0.0:%s", cfg.Port)
	slog.Info("starting the HTTP server", "addr", addr, "env", cfg.Env)

	srv := &http.Server{
		Addr:           addr,
		Handler:        r,
		ReadTimeout:    15 * time.Second,
		WriteTimeout:   15 * time.Second,
		IdleTimeout:    60 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	// Graceful shutdown: on SIGINT / SIGTERM, stop accepting new connections and
	// wait for in-flight requests to finish (up to the timeout).
	//
	// [Ja] グレースフルシャットダウン。SIGINT / SIGTERM を受けたら新規接続の
	// 受け付けを止め、処理中のリクエストの完了を (タイムアウトまで) 待ちます。
	shutdownDone := make(chan struct{})
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		slog.Info("received a shutdown signal")

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			slog.Error("failed to shut down the server", "error", err)
		}
		close(shutdownDone)
	}()

	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("failed to start the server", "error", err)
		os.Exit(1)
	}

	// Wait for the in-flight requests to be drained before exiting. Without this,
	// main would return as soon as ListenAndServe reports ErrServerClosed, while
	// srv.Shutdown is still draining connections in the goroutine.
	//
	// [Ja] 終了する前に処理中リクエストのドレイン完了を待つ。これが無いと、
	// goroutine 内の srv.Shutdown がまだ接続をドレインしている最中でも、
	// ListenAndServe が ErrServerClosed を返した時点で main が返ってしまう。
	<-shutdownDone
	slog.Info("the server has stopped")
}
