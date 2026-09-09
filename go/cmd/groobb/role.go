package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/usecase"
)

// roleChange is what an invocation does to the assignment between a user and a
// role. Resolving the word the command line names to the function that performs
// the change, rather than carrying it through to a second switch, leaves no
// branch that can only be reached with an action this command has already
// rejected; runMigrate resolves its direction the same way.
//
// [Ja] roleChange は、実行が利用者とロールの割当に対して行うことです。コマンドラインが
// 名指しする語を 2 つ目の switch へ持ち回すのではなく、変更を行う関数へ解決してしまう
// ことで、本コマンドが既に弾いた動作でしか到達できない分岐を残さずに済みます。runMigrate も
// 方向を同じ形で解決しています。
type roleChange func(ctx context.Context, db *database.DB, userID model.UserID, name model.RoleName) error

// runRole gives a user a role or takes one away, and returns the process exit
// code. It is how the first administrator is appointed: an instance where nobody
// is one yet has no screen that can hand the role out, and whoever runs this
// holds the database file, which is already the whole of the community's data.
//
// The subcommand is not confined to development. Appointing a successor or
// stepping in when the last administrator is gone are things a production
// instance has to be able to do, so this is one of the commands that run against
// a live database.
//
// The diagnostics a failure produces go through logger rather than the
// package-level one, so that what an invocation reported is read back where it
// was called from. The usage stays on stderr, the stream the other subcommands
// write theirs to.
//
// [Ja] runRole は利用者へロールを与える、または取り上げ、プロセスの終了コードを返します。
// 最初の管理者はこれによって立ちます。まだ誰も管理者でないインスタンスにはロールを配れる
// 画面が無く、これを実行する人はデータベースファイルを手にしています。それはすでに
// コミュニティのデータのすべてです。
//
// 本サブコマンドは開発環境に限定しません。後任を立てることも、最後の管理者が失われたときに
// 立て直すことも、本番のインスタンスが行えなければならないことであり、これは稼働中の
// データベースに対して実行するコマンドの 1 つです。
//
// 失敗が生む診断は、パッケージレベルのものではなく logger を通します。実行が何を報告した
// のかを、それを呼んだ場所から読み戻せるようにするためです。usage は他のサブコマンドが
// 書き込むのと同じく stderr に残します。
func runRole(ctx context.Context, args []string, stderr io.Writer, logger *slog.Logger) int {
	if len(args) != 3 {
		usageRole(stderr)

		return exitUsage
	}

	var change roleChange

	switch args[0] {
	case "grant":
		change = grantRole
	case "revoke":
		change = revokeRole
	default:
		// The write error is discarded for the same reason as in run.
		//
		// [Ja] 書き込みエラーを捨てる理由は run と同じです。
		_, _ = fmt.Fprintf(stderr, "unknown role action: %q\n\n", args[0])
		usageRole(stderr)

		return exitUsage
	}

	if err := changeUserRole(ctx, args[1], model.RoleName(args[2]), change); err != nil {
		logRoleFailure(ctx, logger, err)

		return 1
	}

	return 0
}

// changeUserRole opens the configured database, resolves the atname to the
// account it names, and hands the change to the UseCase.
//
// The command line names the account by atname because that is what a person
// reads on screen and types, while the UseCase takes the id the assignment is
// written against. An atname nothing carries is reported here rather than by the
// UseCase, so that the operator is told which of the two names they gave was not
// found.
//
// [Ja] changeUserRole は設定されたデータベースを開き、atname をそれが指すアカウントへ
// 解決して、変更を UseCase に委ねます。
//
// コマンドラインがアカウントを atname で名指しするのは、それが人が画面で読み、実際に打つ
// 名前であるためです。一方 UseCase が受け取るのは、割当が書き込まれる先の id です。どの
// アカウントも持たない atname はここで報告します。与えた 2 つの名前のどちらが見つからな
// かったのかを、運用者に伝えるためです。
func changeUserRole(ctx context.Context, atname string, name model.RoleName, change roleChange) error {
	return withConfiguredDatabase(ctx, func(_ *config.Config, db *database.DB) error {
		user, err := repository.NewUserRepository(db).FindByAtname(ctx, atname)
		if err != nil {
			return fmt.Errorf("failed to find the user @%s: %w", atname, err)
		}
		if user == nil {
			return fmt.Errorf("no user has the atname %q", atname)
		}

		return change(ctx, db, user.ID, name)
	})
}

// grantRole gives the user the role.
//
// [Ja] grantRole は利用者へロールを与えます。
func grantRole(ctx context.Context, db *database.DB, userID model.UserID, name model.RoleName) error {
	uc := usecase.NewGrantUserRoleUsecase(
		db.Writer,
		repository.NewRoleRepository(db),
		repository.NewUserRepository(db),
		repository.NewUserRoleRepository(db),
	)

	_, err := uc.Execute(ctx, usecase.GrantUserRoleInput{
		Actor:        usecase.OperatorActor(),
		TargetUserID: userID,
		RoleName:     name,
	})
	return err
}

// revokeRole takes the role away from the user.
//
// [Ja] revokeRole は利用者からロールを取り上げます。
func revokeRole(ctx context.Context, db *database.DB, userID model.UserID, name model.RoleName) error {
	uc := usecase.NewRevokeUserRoleUsecase(
		db.Writer,
		repository.NewRoleRepository(db),
		repository.NewUserRepository(db),
		repository.NewUserRoleRepository(db),
	)

	_, err := uc.Execute(ctx, usecase.RevokeUserRoleInput{
		Actor:        usecase.OperatorActor(),
		TargetUserID: userID,
		RoleName:     name,
	})
	return err
}

// logRoleFailure reports through logger why the change did not happen.
//
// A model.AppError is logged through LogString because its Error method returns
// the message written for a person reading a screen, which says that something
// was not found or was refused without saying what. The operator is the one who
// has to act on the answer, so the internal cause and the metadata are what this
// command writes.
//
// [Ja] logRoleFailure は、変更が行われなかった理由を logger へ報告します。
//
// model.AppError を LogString で出すのは、その Error メソッドが返すのが画面を読む人の
// ために書かれたメッセージであり、何かが見つからない / 拒否されたことは述べても、それが
// 何であるかは述べないためです。その答えに基づいて動くのは運用者であるため、本コマンドが
// 書くのは内部原因とメタデータです。
func logRoleFailure(ctx context.Context, logger *slog.Logger, err error) {
	var appErr *model.AppError
	if errors.As(err, &appErr) {
		logger.ErrorContext(ctx, appErr.LogString())

		return
	}

	logger.ErrorContext(ctx, "failed to change the role of the user", "error", err)
}

// usageRole writes how the role subcommand is invoked to w. It names the two
// actions the way the migrate usage names its two directions, so that the line
// answers what to type next. The role is left as a placeholder because which
// roles exist is a property of the database rather than of the binary. The write
// error is discarded for the same reason as in run.
//
// [Ja] usageRole は role サブコマンドの呼び出し方を w に書きます。migrate の usage が
// 2 つの方向を挙げるのと同じく 2 つの動作を挙げることで、この 1 行が、次に何を打てばよいのか
// に答えるようにしています。ロールをプレースホルダーのままにしているのは、どのロールが
// 存在するかがバイナリではなくデータベースの性質であるためです。書き込みエラーを捨てる理由は
// run と同じです。
func usageRole(w io.Writer) {
	_, _ = fmt.Fprint(w, "usage: groobb role grant|revoke <atname> <role>\n")
}
