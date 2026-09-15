package usecase

import (
	"context"
	"fmt"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
)

// resolveRoleAssignmentは、割当が対象とするロールを解決し、それを受け取る (または
// 失う) 利用者をコミュニティがまだ持っていることを確かめます。付与と剥奪はこの2つの
// 問いを同じく尋ねるため、どのロールも持たない名前も、誰でもないアカウントも、どちらが
// 要求されたかによらず同じ形で答えられます。
//
// どちらの不在も、バリデーションエラーではなくAppErrCodeResourceNotFoundです。ロール名も
// 利用者も、誰かが入力したフィールドではなくリクエストが名指すアドレスから来るものであり、
// メッセージを添えて返すフォームがありません。
//
// 退会したアカウントはここでは不在です。ルックアップがそれを外すためです。去った人に
// ロールを与えれば、誰もサインインできない保持者をコミュニティに置くことになります。
//
// 解決した利用者も併せて返します。呼び出し元が、何が起きたのかを述べるために対象の
// atnameを必要とし、それはここで既に読まれているためです。
func resolveRoleAssignment(
	ctx context.Context,
	roleRepo *repository.RoleRepository,
	userRepo *repository.UserRepository,
	name model.RoleName,
	userID model.UserID,
) (*model.Role, *model.User, error) {
	role, err := roleRepo.FindByName(ctx, name)
	if err != nil {
		return nil, nil, fmt.Errorf("ロールの取得に失敗: %w", err)
	}
	if role == nil {
		return nil, nil, &model.AppError{
			Code:     model.AppErrCodeResourceNotFound,
			UserMsg:  i18n.T(ctx, "error_not_found_message"),
			Internal: fmt.Errorf("ロールが見つからない: role_name=%s", name),
			Metadata: map[string]string{"role_name": string(name)},
		}
	}

	user, err := userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, nil, fmt.Errorf("対象の利用者の取得に失敗: %w", err)
	}
	if user == nil {
		return nil, nil, &model.AppError{
			Code:     model.AppErrCodeResourceNotFound,
			UserMsg:  i18n.T(ctx, "error_not_found_message"),
			Internal: fmt.Errorf("対象の利用者が見つからない: user_id=%s", userID),
			Metadata: map[string]string{"user_id": userID.String()},
		}
	}

	return role, user, nil
}

// holdsRoleは、ユーザーが既にそのロールを持っているかどうかを返します。
//
// リポジトリは呼び出し側のトランザクションに参加していなければなりません (WithTx)。この
// 答えが決めるのは書き込むかどうかであり、その外で読めば、答えとそれが導いた書き込みの
// 間に割当が現れたり消えたりしうるためです。
func holdsRole(
	ctx context.Context,
	userRoleRepo *repository.UserRoleRepository,
	userID model.UserID,
	roleID model.RoleID,
) (bool, error) {
	userRoles, err := userRoleRepo.ListByUserIDs(ctx, []model.UserID{userID})
	if err != nil {
		return false, fmt.Errorf("ロールの割当の取得に失敗: %w", err)
	}

	for _, userRole := range userRoles {
		if userRole.RoleID == roleID {
			return true, nil
		}
	}
	return false, nil
}

// removingLeavesNoAdminは、有効なadminロールの保持者を1人外すと、有効な管理者が
// いなくなるかどうかを返します。この結果は人数に含まれる対象にだけ適用します。停止中・
// 退会済みの保持者は既に除外されており、その割当を外しても人数は減りません。
//
// 最後の管理者を失うことは、画面から取り消せる状態ではありません。管理者を立てるのは管理
// 画面であり、誰もそこを許されなくなるためです。残るのはデータベースファイルに対する
// groobbのサブコマンドであり、ここでの拒否が、コミュニティがそれを必要とせずに済む理由
// です。
//
// 件数は書き込むトランザクションの中で読まなければなりません。拒否を判断した数が、それが
// 許した削除より前に変わらないようにするためです。
func removingLeavesNoAdmin(
	ctx context.Context,
	userRoleRepo *repository.UserRoleRepository,
	adminRoleID model.RoleID,
) (bool, error) {
	count, err := userRoleRepo.CountHoldersByRoleID(ctx, adminRoleID)
	if err != nil {
		return false, fmt.Errorf("管理者の人数の取得に失敗: %w", err)
	}
	return count <= 1, nil
}
