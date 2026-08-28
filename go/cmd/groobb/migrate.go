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

// runMigrate moves the configured database along the migrations embedded in the
// binary and returns the process exit code. It lets development migrate and roll
// back without starting the HTTP server.
//
// The direction is resolved before the configuration is read so that a typo
// fails on its own terms rather than as a missing setting or an unreachable
// database. Resolving it to the function that performs the move, instead of
// carrying the word through to a second switch, leaves no branch that can only
// be reached with a direction this function has already rejected.
//
// [Ja] runMigrate は、設定されたデータベースをバイナリに埋め込まれたマイグレーションに
// 沿って動かし、プロセスの終了コードを返します。開発時に HTTP サーバーを起動せず
// マイグレートとロールバックを行えるようにします。
//
// 方向を設定の読み込みより先に解決するのは、打ち間違いが設定漏れやデータベースへの
// 接続失敗としてではなく、それ自体として失敗するようにするためです。方向を表す語を
// 2 つ目の switch へ持ち回すのではなく、移動を行う関数へ解決してしまうことで、本関数が
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
		// The write error is discarded for the same reason as in run.
		//
		// [Ja] 書き込みエラーを捨てる理由は run と同じです。
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

// migrateDatabase opens the configured database and hands its writer to move.
//
// [Ja] migrateDatabase は設定されたデータベースを開き、その Writer を move に渡します。
func migrateDatabase(ctx context.Context, move func(ctx context.Context, writer *sql.DB) error) error {
	return withConfiguredDatabase(ctx, func(_ *config.Config, db *database.DB) error {
		return move(ctx, db.Writer)
	})
}

// usageMigrate writes how the migrate subcommand is invoked to w. The write
// error is discarded for the same reason as in run.
//
// [Ja] usageMigrate は migrate サブコマンドの呼び出し方を w に書きます。書き込み
// エラーを捨てる理由は run と同じです。
func usageMigrate(w io.Writer) {
	_, _ = fmt.Fprint(w, "usage: groobb migrate up|down\n")
}
