package usecase

import (
	"context"
	"fmt"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
)

// resolveRoleAssignment resolves the role an assignment is about and confirms
// that the user receiving or losing it is someone the community still has. Both
// granting and revoking ask the same two questions, so a name nothing carries
// and an account nobody is are answered the same way whichever of them was
// requested.
//
// Either being absent is AppErrCodeResourceNotFound rather than a validation
// error: the role name and the user come from the address a request names, not
// from a field someone filled in, so there is no form to send back with the
// message.
//
// A withdrawn account is absent here, because the lookup leaves it out. Giving a
// role to someone who has left would put a holder into the community that nobody
// can sign in as.
//
// The user that was resolved is handed back as well, since the caller needs the
// target's atname to say what happened, and it has already been read here.
//
// [Ja] resolveRoleAssignment は、割当が対象とするロールを解決し、それを受け取る (または
// 失う) 利用者をコミュニティがまだ持っていることを確かめます。付与と剥奪はこの 2 つの
// 問いを同じく尋ねるため、どのロールも持たない名前も、誰でもないアカウントも、どちらが
// 要求されたかによらず同じ形で答えられます。
//
// どちらの不在も、バリデーションエラーではなく AppErrCodeResourceNotFound です。ロール名も
// 利用者も、誰かが入力したフィールドではなくリクエストが名指すアドレスから来るものであり、
// メッセージを添えて返すフォームがありません。
//
// 退会したアカウントはここでは不在です。ルックアップがそれを外すためです。去った人に
// ロールを与えれば、誰もサインインできない保持者をコミュニティに置くことになります。
//
// 解決した利用者も併せて返します。呼び出し元が、何が起きたのかを述べるために対象の
// atname を必要とし、それはここで既に読まれているためです。
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

// holdsRole reports whether the user already holds the given role.
//
// The repository must be enlisted in the caller's transaction (WithTx), because
// what the answer decides is whether to write: read outside it, an assignment
// could appear or disappear between the answer and the write it led to.
//
// [Ja] holdsRole は、ユーザーが既にそのロールを持っているかどうかを返します。
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

// removingLeavesNoAdmin reports whether taking the admin role away from one of
// its holders would leave the community with nobody holding it. The caller has
// confirmed that the person it is about to remove is a holder, so the role
// having a single holder left is that person.
//
// Losing the last administrator is not a state anyone can undo from a screen:
// the admin screens are what appoints an administrator, and nobody would be
// admitted to them. What remains is the groobb subcommand against the database
// file, so the refusal here is what keeps the community from needing it.
//
// The count must be read inside the transaction that writes, so that the number
// the refusal is decided against cannot change before the removal it admits.
//
// [Ja] removingLeavesNoAdmin は、admin ロールを保持者の 1 人から取り上げると、誰もそれを
// 持たない状態になるかどうかを返します。呼び出し側は、これから取り上げる相手が保持者で
// あることを確かめているため、ロールの保持者が 1 人だけならそれはその人です。
//
// 最後の管理者を失うことは、画面から取り消せる状態ではありません。管理者を立てるのは管理
// 画面であり、誰もそこを許されなくなるためです。残るのはデータベースファイルに対する
// groobb のサブコマンドであり、ここでの拒否が、コミュニティがそれを必要とせずに済む理由
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
