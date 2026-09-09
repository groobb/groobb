package usecase

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
)

// GrantUserRoleUsecase gives someone a role. It is what the admin screens' grant
// button and the groobb role grant subcommand both go through, so who may hand
// out a role is decided in one place however the request arrived.
//
// [Ja] GrantUserRoleUsecase は誰かにロールを与えます。管理画面の付与のボタンと
// groobb role grant のサブコマンドはどちらもここを通るため、誰がロールを配ってよいかは、
// 要求がどこから届いても 1 箇所で決まります。
type GrantUserRoleUsecase struct {
	writer       *sql.DB
	roleRepo     *repository.RoleRepository
	userRepo     *repository.UserRepository
	userRoleRepo *repository.UserRoleRepository
}

// NewGrantUserRoleUsecase builds a GrantUserRoleUsecase from the write pool and
// the repositories it reads and persists through.
//
// [Ja] NewGrantUserRoleUsecase は書き込み用プールと、読み書きに使うリポジトリから
// GrantUserRoleUsecase を構築します。
func NewGrantUserRoleUsecase(
	writer *sql.DB,
	roleRepo *repository.RoleRepository,
	userRepo *repository.UserRepository,
	userRoleRepo *repository.UserRoleRepository,
) *GrantUserRoleUsecase {
	return &GrantUserRoleUsecase{
		writer:       writer,
		roleRepo:     roleRepo,
		userRepo:     userRepo,
		userRoleRepo: userRoleRepo,
	}
}

// GrantUserRoleInput is the input to Execute. Actor is who is handing the role
// out, TargetUserID the account receiving it, and RoleName the role by the name
// every instance addresses it with.
//
// [Ja] GrantUserRoleInput は Execute の入力です。Actor はロールを渡す側、TargetUserID は
// それを受け取るアカウント、RoleName はどのインスタンスでも同じ名前で指されるロールです。
type GrantUserRoleInput struct {
	Actor        Actor
	TargetUserID model.UserID
	RoleName     model.RoleName
}

// GrantUserRoleOutput names the account the role was handed to, as the account
// itself spells its name. The screen answers with the listing and says what it
// did there, and a listing of rows carrying the same words on every button needs
// the answer to name the row that was pressed.
//
// [Ja] GrantUserRoleOutput は、ロールを渡した相手のアカウントを、そのアカウント自身が
// 綴る名前で名指します。画面は一覧で応答し、そこで何をしたのかを述べます。どの行の
// ボタンも同じ言葉を載せる一覧では、応答が、押された行を名指す必要があるためです。
type GrantUserRoleOutput struct {
	TargetAtname string
}

// Execute gives the role to the target user.
//
// Permission, the role and the target are resolved before the transaction, so a
// request that is refused or names nothing costs no write lock. What the
// transaction reads is the assignment itself, which decides whether there is
// anything to write.
//
// [Ja] Execute は対象の利用者へロールを与えます。
//
// 権限・ロール・対象はトランザクションの前で解決するため、拒否される要求も、何も名指して
// いない要求も、書き込みロックを消費しません。トランザクションが読むのは割当そのもので、
// それが書くものがあるかどうかを決めます。
func (uc *GrantUserRoleUsecase) Execute(ctx context.Context, input GrantUserRoleInput) (*GrantUserRoleOutput, error) {
	communityPolicy, err := resolveCommunityPolicy(ctx, uc.roleRepo, input.Actor)
	if err != nil {
		return nil, err
	}
	if !communityPolicy.CanGrantUserRole() {
		return nil, &model.AppError{
			Code:     model.AppErrCodeForbidden,
			UserMsg:  i18n.T(ctx, "error_forbidden_message"),
			Internal: fmt.Errorf("ロールを付与する権限がない: target_user_id=%s role_name=%s", input.TargetUserID, input.RoleName),
			Metadata: map[string]string{"target_user_id": input.TargetUserID.String(), "role_name": string(input.RoleName)},
		}
	}

	role, target, err := resolveRoleAssignment(ctx, uc.roleRepo, uc.userRepo, input.RoleName, input.TargetUserID)
	if err != nil {
		return nil, err
	}

	if err := uc.grant(ctx, input.TargetUserID, role.ID); err != nil {
		return nil, err
	}

	return &GrantUserRoleOutput{TargetAtname: target.Atname}, nil
}

// grant writes the assignment, having first confirmed inside the same
// transaction that the user does not already hold the role.
//
// Holding it already is success rather than a conflict: what the request asked
// for is that the person hold the role, and they do. Two administrators pressing
// the same button end up with the same community, and a subcommand run twice
// says the same thing twice.
//
// The UNIQUE (user_id, role_id) constraint stands behind the confirmation rather
// than in place of it. Nothing else may write between the read and the insert
// while this transaction holds the write lock, so a rejection there means an
// assignment this transaction did not see, which is again the state the request
// asked for.
//
// [Ja] grant は、ユーザーがまだそのロールを持たないことを同じトランザクションの中で確かめて
// から割当を書き込みます。
//
// 既に持っていることは競合ではなく成功です。要求が求めたのはその人がロールを持っている
// ことであり、実際に持っているためです。2 人の管理者が同じボタンを押しても行き着く
// コミュニティは同じであり、2 度実行したサブコマンドは同じことを 2 度述べます。
//
// UNIQUE (user_id, role_id) 制約は、確認の代わりではなくその後ろに立ちます。この
// トランザクションが書き込みロックを保持している間、読み取りと挿入の間に他が書くことは
// ないため、そこでの拒否が意味するのはこのトランザクションが見なかった割当であり、それも
// また要求が求めた状態です。
func (uc *GrantUserRoleUsecase) grant(ctx context.Context, userID model.UserID, roleID model.RoleID) error {
	tx, err := uc.writer.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("トランザクションの開始に失敗: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	userRoleRepo := uc.userRoleRepo.WithTx(tx)

	holds, err := holdsRole(ctx, userRoleRepo, userID, roleID)
	if err != nil {
		return err
	}
	if holds {
		return nil
	}

	if _, err := userRoleRepo.Create(ctx, repository.CreateUserRoleInput{
		UserID: userID,
		RoleID: roleID,
	}); err != nil {
		if repository.IsUniqueViolation(err) {
			return nil
		}
		return fmt.Errorf("ロールの割当の作成に失敗: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("トランザクションのコミットに失敗: %w", err)
	}
	return nil
}
