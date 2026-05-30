// Package database provides helpers to establish the PostgreSQL connection
// pool that is shared across the application.
//
// [Ja] database パッケージは、アプリケーション全体で共有する PostgreSQL の
// 接続プールを確立するためのヘルパーを提供します。
package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// New creates a pgx connection pool from the given connection string and
// verifies connectivity with a ping before returning. Pinging on startup makes
// a misconfigured or unreachable database fail fast instead of surfacing as
// errors on the first request. The caller owns the returned pool and must close
// it.
//
// [Ja] New は与えられた接続文字列から pgx の接続プールを作成し、返す前に ping で
// 疎通を確認します。起動時に ping することで、設定ミスや接続不能なデータベースを
// 最初のリクエストでのエラーとしてではなく早期に検知できます。返したプールは
// 呼び出し側の所有物であり、クローズの責務も呼び出し側にあります。
func New(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create the connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping the database: %w", err)
	}

	return pool, nil
}
