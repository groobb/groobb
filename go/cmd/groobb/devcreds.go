package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/groobb/groobb/go/internal/seed"
)

// runDevCredentialsは、シードが作成したアカウント1件のサインイン用資格情報を
// 出力し、プロセスの終了コードを返します。
//
// アカウントは名簿で持つ役割で指定します。これは生成器がそのアカウントを名指しするときの
// 名前でもあります。位置ではなく役割で指定することが、名簿の手前に誰かが足されたときにも、
// 同じ指定が同じアカウントを指し続ける理由になります。
//
// 2つの値はメールアドレスを先に、1行ずつ標準出力へ書きます。シェルが何も解釈せずに変数へ
// 読み込めるようにするためで、scripts/browse.shはそのように読みます。
func runDevCredentials(
	ctx context.Context,
	args []string,
	appEnv string,
	findCredentials func(role string) (*seed.Credentials, error),
	stdout io.Writer,
	stderr io.Writer,
) int {
	if len(args) != 1 {
		usageDevCredentials(stderr)

		return exitUsage
	}

	credentials, err := devCredentials(appEnv, args[0], findCredentials)
	if err != nil {
		slog.ErrorContext(ctx, "failed to find the credentials of the development account", "error", err)

		return 1
	}

	if _, err := fmt.Fprintf(stdout, "%s\n%s\n", credentials.Email, credentials.Password); err != nil {
		slog.ErrorContext(ctx, "failed to write the credentials", "error", err)

		return 1
	}

	return 0
}

// devCredentialsは、役割を引く前に生のAPP_ENVを検査します。ガードが見るのが、
// config.Loadが未設定時に補う開発環境の既定値ではなく、環境が実際に持っている値である
// ようにするためです。引くこと自体に設定は要りません。名簿はデータベースではなくファイル
// であるためです。
func devCredentials(
	appEnv string,
	role string,
	findCredentials func(role string) (*seed.Credentials, error),
) (*seed.Credentials, error) {
	if err := seed.EnsureDevEnv(appEnv); err != nil {
		return nil, err
	}

	return findCredentials(role)
}

// usageDevCredentialsはdevcredsサブコマンドの呼び出し方をwに書きます。migrateの
// usageが2つの方向を挙げるのと同じく、本サブコマンドが受け付ける役割を挙げることで、
// この1行が、次に何を打てばよいのかに答えるようにしています。書き込みエラーを捨てる理由は
// runと同じです。
func usageDevCredentials(w io.Writer) {
	_, _ = fmt.Fprintf(w, "usage: groobb devcreds %s\n", strings.Join(seed.SignInRoles(), "|"))
}
