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

// roleChangeは、実行が利用者とロールの割当に対して行うことです。コマンドラインが
// 名指しする語を2つ目のswitchへ持ち回すのではなく、変更を行う関数へ解決してしまう
// ことで、本コマンドが既に弾いた動作でしか到達できない分岐を残さずに済みます。runMigrateも
// 方向を同じ形で解決しています。
type roleChange func(ctx context.Context, db *database.DB, userID model.UserID, name model.RoleName) error

// runRoleは利用者へロールを与える、または取り上げ、プロセスの終了コードを返します。
// 最初の管理者はこれによって立ちます。まだ誰も管理者でないインスタンスにはロールを配れる
// 画面が無く、これを実行する人はデータベースファイルを手にしています。それはすでに
// コミュニティのデータのすべてです。
//
// 本サブコマンドは開発環境に限定しません。後任を立てることも、最後の管理者が失われたときに
// 立て直すことも、本番のインスタンスが行えなければならないことであり、これは稼働中の
// データベースに対して実行するコマンドの1つです。
//
// 失敗が生む診断は、パッケージレベルのものではなくloggerを通します。実行が何を報告した
// のかを、それを呼んだ場所から読み戻せるようにするためです。usageは他のサブコマンドが
// 書き込むのと同じくstderrに残します。
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
		// 書き込みエラーを捨てる理由はrunと同じです。
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

// changeUserRoleは設定されたデータベースを開き、atnameをそれが指すアカウントへ
// 解決して、変更をUseCaseに委ねます。
//
// コマンドラインがアカウントをatnameで名指しするのは、それが人が画面で読み、実際に打つ
// 名前であるためです。一方UseCaseが受け取るのは、割当が書き込まれる先のidです。どの
// アカウントも持たないatnameはここで報告します。与えた2つの名前のどちらが見つからな
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

// grantRoleは利用者へロールを与えます。
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

// revokeRoleは利用者からロールを取り上げます。
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

// logRoleFailureは、変更が行われなかった理由をloggerへ報告します。
//
// model.AppErrorをLogStringで出すのは、そのErrorメソッドが返すのが画面を読む人の
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

// usageRoleはroleサブコマンドの呼び出し方をwに書きます。migrateのusageが
// 2つの方向を挙げるのと同じく2つの動作を挙げることで、この1行が、次に何を打てばよいのか
// に答えるようにしています。ロールをプレースホルダーのままにしているのは、どのロールが
// 存在するかがバイナリではなくデータベースの性質であるためです。書き込みエラーを捨てる理由は
// runと同じです。
func usageRole(w io.Writer) {
	_, _ = fmt.Fprint(w, "usage: groobb role grant|revoke <atname> <role>\n")
}
