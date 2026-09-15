package main

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log/slog"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/database"
)

// runMigrateは、設定されたデータベースをバイナリに埋め込まれたマイグレーションに
// 沿って動かし、プロセスの終了コードを返します。開発時にHTTPサーバーを起動せず
// マイグレートとロールバックを行えるようにします。
//
// 方向を設定の読み込みより先に解決するのは、打ち間違いが設定漏れやデータベースへの
// 接続失敗としてではなく、それ自体として失敗するようにするためです。方向を表す語を
// 2つ目のswitchへ持ち回すのではなく、移動を行う関数へ解決してしまうことで、本関数が
// 既に弾いた方向でしか到達できない分岐を残さずに済みます。
func runMigrate(ctx context.Context, args []string, stderr io.Writer) int {
	if len(args) != 1 {
		usageMigrate(stderr)

		return exitUsage
	}

	var move func(ctx context.Context, writer *sql.DB) error

	switch args[0] {
	case "up":
		move = database.Migrate
	case "down":
		move = database.Rollback
	default:
		// 書き込みエラーを捨てる理由はrunと同じです。
		_, _ = fmt.Fprintf(stderr, "unknown migration direction: %q\n\n", args[0])
		usageMigrate(stderr)

		return exitUsage
	}

	if err := migrateDatabase(ctx, move); err != nil {
		slog.ErrorContext(ctx, "failed to migrate the database", "error", err)

		return 1
	}

	return 0
}

// migrateDatabaseは設定されたデータベースを開き、そのWriterをmoveに渡します。
func migrateDatabase(ctx context.Context, move func(ctx context.Context, writer *sql.DB) error) error {
	return withConfiguredDatabase(ctx, func(_ *config.Config, db *database.DB) error {
		return move(ctx, db.Writer)
	})
}

// usageMigrateはmigrateサブコマンドの呼び出し方をwに書きます。書き込み
// エラーを捨てる理由はrunと同じです。
func usageMigrate(w io.Writer) {
	_, _ = fmt.Fprint(w, "usage: groobb migrate up|down\n")
}
