// Command migrate applies the database migrations embedded in the binary to the
// configured database. It lets development migrate and roll back without
// starting the HTTP server.
//
// [Ja] migrate コマンドは、バイナリに埋め込まれたマイグレーションを設定された
// データベースへ適用します。開発時に HTTP サーバーを起動せずマイグレートと
// ロールバックを行えるようにします。
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/database"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		slog.Error("failed to run the migration command", "error", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: migrate up|down")
	}

	// The subcommand is checked before the configuration is read so that a typo
	// fails on its own terms rather than as a missing setting or an unreachable
	// database.
	//
	// [Ja] サブコマンドを設定の読み込みより先に検査するのは、打ち間違いが設定漏れや
	// データベースへの接続失敗としてではなく、それ自体として失敗するようにするため。
	if args[0] != "up" && args[0] != "down" {
		return unknownSubcommandError(args[0])
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load the configuration: %w", err)
	}

	// The bounded context guards only the initial connect and ping, as it does
	// in the server; the migration itself runs without a deadline of its own.
	//
	// [Ja] タイムアウト付きの context は、サーバーと同じく最初の接続と ping だけを
	// 制御する。マイグレーション自体には固有の期限を設けない。
	connectCtx, connectCancel := context.WithTimeout(context.Background(), 10*time.Second)
	db, err := database.Open(connectCtx, cfg.DatabasePath)
	connectCancel()
	if err != nil {
		return fmt.Errorf("failed to open the database: %w", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			slog.Error("failed to close the database", "error", err)
		}
	}()

	ctx := context.Background()

	switch args[0] {
	case "up":
		return database.Migrate(ctx, db.Writer)
	case "down":
		return database.Rollback(ctx, db.Writer)
	default:
		return unknownSubcommandError(args[0])
	}
}

// unknownSubcommandError reports that name is not a subcommand this command
// knows. Both the check above and the dispatch reject an unknown subcommand, so
// the message is built in one place to keep the two from drifting apart.
//
// [Ja] unknownSubcommandError は name が本コマンドの知らないサブコマンドであることを
// 報告します。前段のチェックと振り分けの両方が未知のサブコマンドを弾くため、両者が
// 食い違わないようメッセージを 1 箇所で組み立てます。
func unknownSubcommandError(name string) error {
	return fmt.Errorf("unknown subcommand %q: use \"up\" or \"down\"", name)
}
