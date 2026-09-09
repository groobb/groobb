package usecase

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
)

// RevokeUserRoleUsecase takes a role away from someone. Taking one's own admin
// role away is admitted like any other, so an administrator stepping down does
// not need a second administrator to do it for them; what is refused is leaving
// the community with no administrator at all.
//
// [Ja] RevokeUserRoleUsecase は誰かからロールを取り上げます。自分自身の admin ロールを
// 外すことも他と同じく許されるため、管理者を降りる人がそのために別の管理者を必要とする
// ことはありません。拒否されるのは、コミュニティを管理者のいない状態にすることです。
type RevokeUserRoleUsecase struct {
	writer       *sql.DB
	roleRepo     *repository.RoleRepository
	userRepo     *repository.UserRepository
	userRoleRepo *repository.UserRoleRepository
}

// NewRevokeUserRoleUsecase builds a RevokeUserRoleUsecase from the write pool
// and the repositories it reads and persists through.
//
// [Ja] NewRevokeUserRoleUsecase は書き込み用プールと、読み書きに使うリポジトリから
// RevokeUserRoleUsecase を構築します。
func NewRevokeUserRoleUsecase(
	writer *sql.DB,
	roleRepo *repository.RoleRepository,
	userRepo *repository.UserRepository,
	userRoleRepo *repository.UserRoleRepository,
) *RevokeUserRoleUsecase {
	return &RevokeUserRoleUsecase{
		writer:       writer,
		roleRepo:     roleRepo,
		userRepo:     userRepo,
		userRoleRepo: userRoleRepo,
	}
}

// RevokeUserRoleInput is the input to Execute. Actor is who is taking the role
// away, TargetUserID the account losing it, and RoleName the role by the name
// every instance addresses it with.
//
// [Ja] RevokeUserRoleInput は Execute の入力です。Actor はロールを取り上げる側、
// TargetUserID はそれを失うアカウント、RoleName はどのインスタンスでも同じ名前で指される
// ロールです。
type RevokeUserRoleInput struct {
	Actor        Actor
	TargetUserID model.UserID
	RoleName     model.RoleName
}

// RevokeUserRoleOutput names the account the role was taken from, as the account
// itself spells its name, for the reason GrantUserRoleOutput carries it.
//
// [Ja] RevokeUserRoleOutput は、ロールを取り上げた相手のアカウントを、そのアカウント
// 自身が綴る名前で名指します。理由は GrantUserRoleOutput がそれを運ぶ理由と同じです。
type RevokeUserRoleOutput struct {
	TargetAtname string
}

// Execute takes the role away from the target user.
//
// Permission, the role and the target are resolved before the transaction, so a
// request that is refused or names nothing costs no write lock. What the
// transaction reads is the assignment and, for the admin role, how many people
// still hold it: both decide whether the removal happens.
//
// [Ja] Execute は対象の利用者からロールを取り上げます。
//
// 権限・ロール・対象はトランザクションの前で解決するため、拒否される要求も、何も名指して
// いない要求も、書き込みロックを消費しません。トランザクションが読むのは割当と、admin
// ロールについてはまだ何人がそれを持つかであり、どちらも削除が起こるかどうかを決めます。
func (uc *RevokeUserRoleUsecase) Execute(ctx context.Context, input RevokeUserRoleInput) (*RevokeUserRoleOutput, error) {
	communityPolicy, err := resolveCommunityPolicy(ctx, uc.roleRepo, input.Actor)
	if err != nil {
		return nil, err
	}
	if !communityPolicy.CanRevokeUserRole() {
		return nil, &model.AppError{
			Code:     model.AppErrCodeForbidden,
			UserMsg:  i18n.T(ctx, "error_forbidden_message"),
			Internal: fmt.Errorf("ロールを剥奪する権限がない: target_user_id=%s role_name=%s", input.TargetUserID, input.RoleName),
			Metadata: map[string]string{"target_user_id": input.TargetUserID.String(), "role_name": string(input.RoleName)},
		}
	}

	role, target, err := resolveRoleAssignment(ctx, uc.roleRepo, uc.userRepo, input.RoleName, input.TargetUserID)
	if err != nil {
		return nil, err
	}

	if err := uc.revoke(ctx, input.TargetUserID, role); err != nil {
		return nil, err
	}

	return &RevokeUserRoleOutput{TargetAtname: target.Atname}, nil
}

// revoke removes the assignment, having first confirmed inside the same
// transaction that the user holds the role and that removing it leaves the
// community an administrator.
//
// Not holding it is success rather than a missing resource: what the request
// asked for is that the person not hold the role, and they do not. A revoke
// arriving twice therefore says the same thing twice, as does one racing another
// for the same assignment.
//
// [Ja] revoke は、ユーザーがそのロールを持つこと、そしてそれを外してもコミュニティに管理者
// が残ることを同じトランザクションの中で確かめてから、割当を削除します。
//
// 持っていないことは、リソースの不在ではなく成功です。要求が求めたのはその人がロールを
// 持っていないことであり、実際に持っていないためです。したがって 2 度届いた剥奪は同じことを
// 2 度述べ、同じ割当を奪い合った 2 つの剥奪も同じく述べます。
func (uc *RevokeUserRoleUsecase) revoke(ctx context.Context, userID model.UserID, role *model.Role) error {
	tx, err := uc.writer.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("トランザクションの開始に失敗: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	userRoleRepo := uc.userRoleRepo.WithTx(tx)

	holds, err := holdsRole(ctx, userRoleRepo, userID, role.ID)
	if err != nil {
		return err
	}
	if !holds {
		return nil
	}

	if role.Name == model.RoleNameAdmin {
		leavesNoAdmin, err := removingLeavesNoAdmin(ctx, userRoleRepo, role.ID)
		if err != nil {
			return err
		}
		if leavesNoAdmin {
			return &model.AppError{
				Code:     model.AppErrCodeConflict,
				UserMsg:  i18n.T(ctx, "validation_user_role_last_admin"),
				Internal: fmt.Errorf("最後の管理者からの剥奪: target_user_id=%s", userID),
				Metadata: map[string]string{"target_user_id": userID.String()},
			}
		}
	}

	if err := userRoleRepo.DeleteByUserIDAndRoleID(ctx, userID, role.ID); err != nil {
		return fmt.Errorf("ロールの割当の削除に失敗: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("トランザクションのコミットに失敗: %w", err)
	}
	return nil
}
