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
// connections. Worker registrations (and the dependencies they need, built from
// cfg) are added here as jobs are introduced in later tasks; for now the worker
// set is intentionally empty, so the client can poll its queue but has nothing
// to run yet.
//
// [Ja] NewClient は専用の接続プール上に River クライアントを構築する。プールは
// バックグラウンドジョブ処理専用で、アプリのリクエスト処理用プールとは分離し、ジョブと
// HTTP トラフィックが同じ接続を奪い合わないようにする。ワーカーの登録 (および cfg から
// 構築するその依存) は、後続タスクでジョブが導入されるのに合わせてここに追加する。
// 現時点ではワーカー集合は意図的に空であり、クライアントはキューをポーリングできるが
// まだ実行するものを持たない。
func NewClient(ctx context.Context, databaseURL string) (*Client, error) {
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

	workers := river.NewWorkers()

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
