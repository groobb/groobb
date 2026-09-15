package usecase

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
)

// GrantUserRoleUsecaseは誰かにロールを与えます。管理画面の付与のボタンと
// groobb role grantのサブコマンドはどちらもここを通るため、誰がロールを配ってよいかは、
// 要求がどこから届いても1箇所で決まります。
type GrantUserRoleUsecase struct {
	writer       *sql.DB
	roleRepo     *repository.RoleRepository
	userRepo     *repository.UserRepository
	userRoleRepo *repository.UserRoleRepository
}

// NewGrantUserRoleUsecaseは書き込み用プールと、読み書きに使うリポジトリから
// GrantUserRoleUsecaseを構築します。
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

// GrantUserRoleInputはExecuteの入力です。Actorはロールを渡す側、TargetUserIDは
// それを受け取るアカウント、RoleNameはどのインスタンスでも同じ名前で指されるロールです。
type GrantUserRoleInput struct {
	Actor        Actor
	TargetUserID model.UserID
	RoleName     model.RoleName
}

// GrantUserRoleOutputは、ロールを渡した相手のアカウントを、そのアカウント自身が
// 綴る名前で名指します。画面は一覧で応答し、そこで何をしたのかを述べます。どの行の
// ボタンも同じ言葉を載せる一覧では、応答が、押された行を名指す必要があるためです。
type GrantUserRoleOutput struct {
	TargetAtname string
}

// Executeは対象の利用者へロールを与えます。
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

// grantは、ユーザーがまだそのロールを持たないことを同じトランザクションの中で確かめて
// から割当を書き込みます。
//
// 既に持っていることは競合ではなく成功です。要求が求めたのはその人がロールを持っている
// ことであり、実際に持っているためです。2人の管理者が同じボタンを押しても行き着く
// コミュニティは同じであり、2度実行したサブコマンドは同じことを2度述べます。
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
