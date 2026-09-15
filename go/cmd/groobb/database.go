package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/database"
)

// connectTimeoutは、1つの処理を行って終了するサブコマンドの、最初の接続とpingを
// 制御します。その後の処理には、サーバーと同じく固有の期限を設けません。
const connectTimeout = 10 * time.Second

// withConfiguredDatabaseは設定を読み込み、そこで指定されたデータベースを開いて、
// 両方をrunへ渡します。データベースはrunが返った時点でクローズします。
//
// クローズを各サブコマンドではなくここで行うのは、サブコマンドが開いたままにできないよう
// にするためです。結果を終了コードではなくエラーとして返すのは、クローズが必ず行われる
// ようにするためです。os.Exitはdeferした処理を飛ばしてしまいます。
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
