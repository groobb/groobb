package usecase

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
)

// RevokeUserRoleUsecaseは誰かからロールを取り上げます。自分自身のadminロールを
// 外すことも他と同じく許されるため、管理者を降りる人がそのために別の管理者を必要とする
// ことはありません。拒否されるのは、コミュニティを管理者のいない状態にすることです。
type RevokeUserRoleUsecase struct {
	writer       *sql.DB
	roleRepo     *repository.RoleRepository
	userRepo     *repository.UserRepository
	userRoleRepo *repository.UserRoleRepository
}

// NewRevokeUserRoleUsecaseは書き込み用プールと、読み書きに使うリポジトリから
// RevokeUserRoleUsecaseを構築します。
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

// RevokeUserRoleInputはExecuteの入力です。Actorはロールを取り上げる側、
// TargetUserIDはそれを失うアカウント、RoleNameはどのインスタンスでも同じ名前で指される
// ロールです。
type RevokeUserRoleInput struct {
	Actor        Actor
	TargetUserID model.UserID
	RoleName     model.RoleName
}

// RevokeUserRoleOutputは、ロールを取り上げた相手のアカウントを、そのアカウント
// 自身が綴る名前で名指します。理由はGrantUserRoleOutputがそれを運ぶ理由と同じです。
type RevokeUserRoleOutput struct {
	TargetAtname string
}

// Executeは対象の利用者からロールを取り上げます。
//
// 権限・ロール・対象はトランザクションの前で解決するため、拒否される要求も、何も名指して
// いない要求も、書き込みロックを消費しません。トランザクションが読むのは割当と、admin
// ロールについては対象の現在の状態と有効な保持者数です。これらから、剥奪によって有効な
// 管理者がいなくなるかどうかを判断します。
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

// revokeは、ユーザーがそのロールを持つこと、そしてそれを外してもコミュニティに管理者
// が残ることを同じトランザクションの中で確かめてから、割当を削除します。
//
// 持っていないことは、リソースの不在ではなく成功です。要求が求めたのはその人がロールを
// 持っていないことであり、実際に持っていないためです。したがって2度届いた剥奪は同じことを
// 2度述べ、同じ割当を奪い合った2つの剥奪も同じく述べます。
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
		// 停止中・退会済みの対象は有効な保持者数に含まれません。停止・解除が同時に
		// 届いても、剥奪が人数を減らすかどうかの判断が変わらないよう、対象はこの
		// トランザクションの中で読みます。
		target, err := uc.userRepo.WithTx(tx).FindByID(ctx, userID)
		if err != nil {
			return fmt.Errorf("対象の利用者の取得に失敗: %w", err)
		}
		if target != nil && target.SuspendedAt == nil {
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
	}

	if err := userRoleRepo.DeleteByUserIDAndRoleID(ctx, userID, role.ID); err != nil {
		return fmt.Errorf("ロールの割当の削除に失敗: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("トランザクションのコミットに失敗: %w", err)
	}
	return nil
}
