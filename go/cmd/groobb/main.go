// groobbコマンドはGroobbのコマンドラインエントリポイントです。serve
// サブコマンドがHTTPサーバーを起動し、migrateサブコマンドがバイナリに埋め込まれた
// マイグレーションの適用とロールバックを行い、roleサブコマンドが利用者へロールを与える / 取り上げ、
// seedサブコマンドが開発用データベースを、その画面を眺めるコミュニティの状態へ作り直し、
// devcredsサブコマンドが、そのseedが作成するアカウント1件のサインイン用資格情報を
// 出力します。
package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/groobb/groobb/go/internal/seed"
)

// exitUsageは、サブコマンドを指定していない / 未知のサブコマンドを指定した
// コマンドラインに対する終了コードです。使用方法の誤りを2、依頼された処理自体の
// 失敗を1とするGoツールチェインやgetoptの慣習に従います。
const exitUsage = 2

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// runはargsをサブコマンドへ振り分け、プロセスの終了コードを返します。
// os.Argsを読んでプロセス自身のストリームへ書くのではなく、引数と2つの出力先を
// 仮引数で受け取るのは、テストプロセスを終了させずに振り分けを検証できるように
// するためです。
//
// 標準出力を通しているのはdevcredsのためで、その標準出力はscripts/browse.shが読む
// 機械可読な契約になっています。振り分けの中でos.Stdoutを直接掴むと、呼び出し側が
// 解釈する唯一のストリームがテストから触れなくなります。
//
// サブコマンド無しのときにserveへ既定することはしません。何も指定しない実行は
// usageを表示して失敗するため、各呼び出し箇所がどのサブコマンドを使うのかを明示する
// ことになります。
func run(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stderr)

		return exitUsage
	}

	switch args[0] {
	case "serve":
		runServe()
	case "migrate":
		return runMigrate(context.Background(), args[1:], stderr)
	case "role":
		return runRole(context.Background(), args[1:], stderr, slog.Default())
	case "seed":
		return runSeed(context.Background(), args[1:], stderr)
	case "devcreds":
		return runDevCredentials(
			context.Background(),
			args[1:],
			os.Getenv("APP_ENV"),
			seed.FindCredentials,
			stdout,
			stderr,
		)
	default:
		// 書き込みエラーは意図的に捨てる。ここは診断情報の出力先そのものであり、
		// その書き込みに失敗したことを報告する先が残っていないためである。何が起きたかは
		// 終了コードで呼び出し側に伝わる。
		_, _ = fmt.Fprintf(stderr, "unknown subcommand: %q\n\n", args[0])
		usage(stderr)

		return exitUsage
	}

	return 0
}

// usageは利用可能なサブコマンドの一覧をwに書きます。書き込みエラーを捨てる
// 理由はrunと同じです。
func usage(w io.Writer) {
	_, _ = fmt.Fprint(w, `usage: groobb <command>

commands:
  serve            start the HTTP server
  migrate up       apply the pending migrations
  migrate down     roll back the most recent migration
  role grant       give a user a role
  role revoke      take a role away from a user
  seed [profile]   rebuild the development database with seed data
  devcreds <role>  print the sign-in credentials of a seeded account
`)
}
