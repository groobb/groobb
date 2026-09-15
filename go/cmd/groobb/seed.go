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

// runSeedは開発用データベースをシードデータで作り直し、プロセスの終了コードを
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

// seedProfileは、コマンドラインが求めるコミュニティの状態を解決します。引数が
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

// seedDatabaseはconfig.Loadが開発環境の既定値を適用する前の生のAPP_ENVを
// 検査し、そのうえでデータベースを開いて処理をinternal/seedに委ねます。
//
// 実行側にもガードがあるのにここでも検査するのは、config.Loadが未設定のAPP_ENVを
// 開発環境として読むためです。そうしないと、APP_ENVを設定しない本番プロセスが、
// 読み込み済みの設定に対する検査へ到達して通過してしまいます。
func seedDatabase(ctx context.Context, appEnv string, profile seed.Profile) error {
	if err := seed.EnsureDevEnv(appEnv); err != nil {
		return err
	}

	return withConfiguredDatabase(ctx, func(cfg *config.Config, db *database.DB) error {
		// 進捗はslogが書いているのと同じ標準エラー出力へ送ります。カウンタと
		// その前後のログ行が、書いた順のまま並ぶようにするためです。このコマンドの標準
		// 出力を読む利用側はない (groobb seedは機械可読な出力を持たない) ため、進捗を
		// そこへ載せなくても取り残される呼び出し側はありません。
		return seed.NewRunner(db, cfg, os.Stderr, profile).Run(ctx)
	})
}

// usageSeedはseedサブコマンドの呼び出し方をwに書きます。devcredsのusageが
// 役割を挙げるのと同じく、本サブコマンドが生成する状態を挙げることで、この1行が、次に
// 何を打てばよいのかに答えるようにしています。書き込みエラーを捨てる理由はrunと同じです。
func usageSeed(w io.Writer) {
	_, _ = fmt.Fprintf(w, "usage: groobb seed [%s]\n", strings.Join(seed.ProfileNames(), "|"))
}
