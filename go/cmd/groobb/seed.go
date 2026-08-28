package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/seed"
)

// runSeed rebuilds the development database with seed data and returns the
// process exit code.
//
// The one argument the subcommand takes is the state of the community to
// generate, and anything else is rejected rather than ignored. A run destroys
// every row it manages, so neither a mistyped `groobb seed --help` nor a
// misspelled profile may read as a request to empty the database and refill it
// with something other than what was asked for.
//
// [Ja] runSeed は開発用データベースをシードデータで作り直し、プロセスの終了コードを
// 返します。
//
// 本サブコマンドが取る唯一の引数は、生成するコミュニティの状態です。それ以外は無視せず
// 拒否します。実行は管理対象の行をすべて破棄するため、打ち間違えた `groobb seed --help` も、
// 綴りを誤ったプロファイルも、データベースを空にして依頼とは別のもので埋め直す依頼として
// 読まれてはならないためです。
func runSeed(ctx context.Context, args []string, stderr io.Writer) int {
	profile, ok := seedProfile(args)
	if !ok {
		usageSeed(stderr)

		return exitUsage
	}

	if err := seedDatabase(ctx, os.Getenv("APP_ENV"), profile); err != nil {
		slog.ErrorContext(ctx, "failed to seed the database", "error", err)

		return 1
	}

	return 0
}

// seedProfile resolves the community state a command line asks for. No argument
// is the mature community, the state every screen is worked on against; the
// first-day state is named when that is what is being checked.
//
// [Ja] seedProfile は、コマンドラインが求めるコミュニティの状態を解決します。引数が
// 無ければ成熟したコミュニティで、これはどの画面もそれに照らして作られている状態です。
// 立ち上げ直後の状態は、それを確かめたいときに名指しします。
func seedProfile(args []string) (seed.Profile, bool) {
	switch len(args) {
	case 0:
		return seed.DefaultProfile(), true
	case 1:
		return seed.FindProfile(args[0])
	default:
		return seed.Profile{}, false
	}
}

// seedDatabase checks the raw APP_ENV before config.Load can apply its
// development default, then opens the database and hands the work to
// internal/seed.
//
// The guard runs here as well as inside the run because config.Load reads an
// unset APP_ENV as development: a production process that never sets it would
// otherwise reach a check made on the loaded configuration and pass it.
//
// [Ja] seedDatabase は config.Load が開発環境の既定値を適用する前の生の APP_ENV を
// 検査し、そのうえでデータベースを開いて処理を internal/seed に委ねます。
//
// 実行側にもガードがあるのにここでも検査するのは、config.Load が未設定の APP_ENV を
// 開発環境として読むためです。そうしないと、APP_ENV を設定しない本番プロセスが、
// 読み込み済みの設定に対する検査へ到達して通過してしまいます。
func seedDatabase(ctx context.Context, appEnv string, profile seed.Profile) error {
	if err := seed.EnsureDevEnv(appEnv); err != nil {
		return err
	}

	return withConfiguredDatabase(ctx, func(cfg *config.Config, db *database.DB) error {
		// Progress goes to stderr, the stream slog already writes to, so that the
		// counter and the log lines around it stay in the order they were
		// written. Nothing reads this command's standard output — groobb seed has
		// no machine-readable output — so keeping the progress off it leaves no
		// caller behind.
		//
		// [Ja] 進捗は slog が書いているのと同じ標準エラー出力へ送ります。カウンタと
		// その前後のログ行が、書いた順のまま並ぶようにするためです。このコマンドの標準
		// 出力を読む利用側はない (groobb seed は機械可読な出力を持たない) ため、進捗を
		// そこへ載せなくても取り残される呼び出し側はありません。
		return seed.NewRunner(db, cfg, os.Stderr, profile).Run(ctx)
	})
}

// usageSeed writes how the seed subcommand is invoked to w. It names the states
// the subcommand generates, the way the devcreds usage names its roles, so that
// the line answers what to type next. The write error is discarded for the same
// reason as in run.
//
// [Ja] usageSeed は seed サブコマンドの呼び出し方を w に書きます。devcreds の usage が
// 役割を挙げるのと同じく、本サブコマンドが生成する状態を挙げることで、この 1 行が、次に
// 何を打てばよいのかに答えるようにしています。書き込みエラーを捨てる理由は run と同じです。
func usageSeed(w io.Writer) {
	_, _ = fmt.Fprintf(w, "usage: groobb seed [%s]\n", strings.Join(seed.ProfileNames(), "|"))
}
