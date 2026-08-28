package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/database"
)

// connectTimeout bounds the initial connect and ping of a subcommand that runs
// one task and exits. The work that follows runs without a deadline of its own,
// as it does in the server.
//
// [Ja] connectTimeout は、1 つの処理を行って終了するサブコマンドの、最初の接続と ping を
// 制御します。その後の処理には、サーバーと同じく固有の期限を設けません。
const connectTimeout = 10 * time.Second

// withConfiguredDatabase loads the configuration, opens the database it names,
// and hands both to run, closing the database once run returns.
//
// The database is closed here rather than by each subcommand so that a
// subcommand cannot leave it open, and the result is returned as an error rather
// than as an exit code so that the close always happens: os.Exit skips deferred
// calls.
//
// [Ja] withConfiguredDatabase は設定を読み込み、そこで指定されたデータベースを開いて、
// 両方を run へ渡します。データベースは run が返った時点でクローズします。
//
// クローズを各サブコマンドではなくここで行うのは、サブコマンドが開いたままにできないよう
// にするためです。結果を終了コードではなくエラーとして返すのは、クローズが必ず行われる
// ようにするためです。os.Exit は defer した処理を飛ばしてしまいます。
func withConfiguredDatabase(ctx context.Context, run func(cfg *config.Config, db *database.DB) error) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load the configuration: %w", err)
	}

	connectCtx, connectCancel := context.WithTimeout(ctx, connectTimeout)
	db, err := database.Open(connectCtx, cfg.DatabasePath)
	connectCancel()
	if err != nil {
		return fmt.Errorf("failed to open the database: %w", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			slog.ErrorContext(ctx, "failed to close the database", "error", err)
		}
	}()

	return run(cfg, db)
}
