// Package worker manages the lifecycle of the River background job client:
// building its connection pool, registering workers, and starting/stopping the
// client. A worker is a thin adapter that converts a job's arguments and calls a
// UseCase; worker implementations are registered here as jobs are introduced in
// later tasks (the first being the email-confirmation job).
//
// [Ja] worker パッケージは River バックグラウンドジョブクライアントのライフサイクル
// (接続プールの構築・ワーカーの登録・起動/停止) を管理する。ワーカーはジョブの引数を
// 変換して UseCase を呼ぶ薄い Adapter であり、その実装は後続タスクでジョブが導入されるのに
// 合わせてここに登録する (最初はメール確認ジョブ)。
package worker

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/email"
	"github.com/groobb/groobb/go/internal/usecase"
)

// Client wraps the River client together with the pgx pool it owns, so the two
// can be started and shut down as a single unit.
//
// [Ja] Client は River クライアントと、それが所有する pgx プールをまとめて保持し、
// 起動と停止を 1 つの単位として扱えるようにする。
type Client struct {
	riverClient *river.Client[pgx.Tx]
	pool        *pgxpool.Pool
}

// NewClient builds the River client on its own connection pool. The pool is
// dedicated to background job processing, separate from the application's
// request-serving pool, so jobs and HTTP traffic do not compete for the same
// connections. Worker-only dependencies are built from cfg inside this function
// (rather than injected) so they stay encapsulated here and never leak into the
// rest of the DI graph; more worker registrations are added here as jobs are
// introduced in later tasks.
//
// [Ja] NewClient は専用の接続プール上に River クライアントを構築する。プールは
// バックグラウンドジョブ処理専用で、アプリのリクエスト処理用プールとは分離し、ジョブと
// HTTP トラフィックが同じ接続を奪い合わないようにする。ワーカー専用の依存は (注入では
// なく) 本関数内で cfg から構築し、ここに閉じ込めて DI グラフの他の部分へ漏らさない。
// 追加のワーカー登録は、後続タスクでジョブが導入されるのに合わせてここに加える。
func NewClient(ctx context.Context, databaseURL string, cfg *config.Config) (*Client, error) {
	poolConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("ワーカー用プール設定の解析に失敗: %w", err)
	}

	// Keep the worker pool small and bounded: background jobs are not
	// latency-critical, and a dedicated pool avoids starving the HTTP pool.
	//
	// [Ja] ワーカー用プールは小さく上限を設ける。バックグラウンドジョブはレイテンシ
	// 重視ではなく、専用プールにすることで HTTP 用プールを枯渇させないようにする。
	poolConfig.MaxConns = 10
	poolConfig.MinConns = 2
	poolConfig.MaxConnLifetime = 5 * time.Minute
	poolConfig.MaxConnIdleTime = 2 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("ワーカー用接続プールの作成に失敗: %w", err)
	}

	// Build the worker-only email dependencies from cfg and register the mail
	// workers (email confirmation and password reset). Constructing the senders
	// here keeps Resend configuration out of main.go's DI graph, where no
	// request-path code needs it; both per-mail senders share the one base
	// ResendSender.
	//
	// [Ja] ワーカー専用のメール依存を cfg から構築し、メールワーカー (メール確認と
	// パスワードリセット) を登録する。ここで sender を構築することで、リクエスト経路の
	// コードが必要としない Resend 設定を main.go の DI グラフから締め出す。メール種別ごとの
	// 2 つの sender は 1 つの基盤 ResendSender を共有する。
	emailSender := email.NewResendSender(cfg.ResendAPIKey, cfg.EmailFrom, cfg.EmailFromName)
	confirmationSender := email.NewConfirmationSender(emailSender)
	sendEmailConfirmationUC := usecase.NewSendEmailConfirmationUsecase(confirmationSender)

	passwordResetSender := email.NewPasswordResetSender(emailSender)
	sendPasswordResetUC := usecase.NewSendPasswordResetUsecase(passwordResetSender)

	workers := river.NewWorkers()
	river.AddWorker(workers, NewSendEmailConfirmationWorker(sendEmailConfirmationUC))
	river.AddWorker(workers, NewSendPasswordResetWorker(sendPasswordResetUC))

	// Logger: slog.Default() routes River's own job-execution and retry logging
	// through the structured logger, so observability is in place before any
	// worker is added (a worker's Work method then only needs to return errors).
	//
	// [Ja] Logger: slog.Default() により River 自身のジョブ実行・リトライログを構造化
	// ロガー経由で出力する。これでワーカー追加前から観測性が確保され、ワーカーの Work
	// メソッドはエラーを返すだけでよくなる。
	riverClient, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Queues:  map[string]river.QueueConfig{river.QueueDefault: {MaxWorkers: 10}},
		Workers: workers,
		Logger:  slog.Default(),
	})
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("ワーカーの River クライアントの作成に失敗: %w", err)
	}

	return &Client{
		riverClient: riverClient,
		pool:        pool,
	}, nil
}

// Start begins fetching and working jobs.
//
// [Ja] Start はジョブの取得と処理を開始する。
func (c *Client) Start(ctx context.Context) error {
	slog.InfoContext(ctx, "River クライアントを起動します")
	return c.riverClient.Start(ctx)
}

// Stop drains in-flight jobs, stops the client, and closes the pool it owns.
//
// [Ja] Stop は実行中のジョブをドレインしてクライアントを停止し、所有するプールを閉じる。
func (c *Client) Stop(ctx context.Context) error {
	slog.InfoContext(ctx, "River クライアントを停止します")
	if err := c.riverClient.Stop(ctx); err != nil {
		return err
	}
	c.pool.Close()
	return nil
}

// Client exposes the underlying River client so it can be wired into the
// Dispatcher (it satisfies dispatcher.JobInserter) for enqueueing jobs.
//
// [Ja] Client は基盤の River クライアントを公開し、ジョブ投入のため Dispatcher に
// 配線できるようにする (dispatcher.JobInserter を満たす)。
func (c *Client) Client() *river.Client[pgx.Tx] {
	return c.riverClient
}
