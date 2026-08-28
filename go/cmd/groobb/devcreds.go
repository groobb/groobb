package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/groobb/groobb/go/internal/seed"
)

// runDevCredentials writes the sign-in credentials of one seeded account and
// returns the process exit code.
//
// The account is named by the role it holds in the roster, which is the name the
// generators reach for it by as well. Naming it by role rather than by position
// is what keeps an invocation pointed at the same account when someone is added
// to the roster ahead of it.
//
// The two values go to standard output one per line, the email first, so that a
// shell can read them into variables without parsing anything; scripts/browse.sh
// does exactly that.
//
// [Ja] runDevCredentials は、シードが作成したアカウント 1 件のサインイン用資格情報を
// 出力し、プロセスの終了コードを返します。
//
// アカウントは名簿で持つ役割で指定します。これは生成器がそのアカウントを名指しするときの
// 名前でもあります。位置ではなく役割で指定することが、名簿の手前に誰かが足されたときにも、
// 同じ指定が同じアカウントを指し続ける理由になります。
//
// 2 つの値はメールアドレスを先に、1 行ずつ標準出力へ書きます。シェルが何も解釈せずに変数へ
// 読み込めるようにするためで、scripts/browse.sh はそのように読みます。
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

// devCredentials checks the raw APP_ENV before looking the role up, so that the
// guard sees what the environment actually holds rather than the development
// default config.Load substitutes for an unset value. The lookup itself needs no
// configuration: the roster is a file, not a database.
//
// [Ja] devCredentials は、役割を引く前に生の APP_ENV を検査します。ガードが見るのが、
// config.Load が未設定時に補う開発環境の既定値ではなく、環境が実際に持っている値である
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

// usageDevCredentials writes how the devcreds subcommand is invoked to w. It
// names the roles the subcommand takes, the way the migrate usage names its two
// directions, so that the line answers what to type next. The write error is
// discarded for the same reason as in run.
//
// [Ja] usageDevCredentials は devcreds サブコマンドの呼び出し方を w に書きます。migrate の
// usage が 2 つの方向を挙げるのと同じく、本サブコマンドが受け付ける役割を挙げることで、
// この 1 行が、次に何を打てばよいのかに答えるようにしています。書き込みエラーを捨てる理由は
// run と同じです。
func usageDevCredentials(w io.Writer) {
	_, _ = fmt.Fprintf(w, "usage: groobb devcreds %s\n", strings.Join(seed.SignInRoles(), "|"))
}
