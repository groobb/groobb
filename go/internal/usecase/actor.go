package usecase

import (
	"context"
	"fmt"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/policy"
	"github.com/groobb/groobb/go/internal/repository"
)

// Actorは、管理系のUseCaseが誰のために動くかを表します。サインイン済みの利用者
// (許されることはその人が持つロールのスコープ) か、データベースファイルに対してgroobbの
// サブコマンドを実行する運用者 (すべてを許される) のどちらかです。
//
// 運用者であることは、利用者idを省くことではなくOperatorActorを呼ぶことで述べます。
// idを伴わずに届いた値は、誰でもない利用者、すなわちロールを1つも持たず何も許されない
// 利用者です。そのため、サインイン済みの利用者を渡し忘れたときに起こるのは、すべての操作が
// 許されることではなく、その操作が拒まれることです。
type Actor struct {
	userID     model.UserID
	isOperator bool
}

// UserActorは、サインイン済みの利用者を表す操作者を返します。idは認証済みの
// セッションから来るものであり、フォームやURLから来るものではありません。誰が操作するか
// を、誰がサインインしたかが決めるようにするためです。
func UserActor(userID model.UserID) Actor {
	return Actor{userID: userID}
}

// OperatorActorは、groobbのサブコマンドを実行する人を表す操作者を返します。
// データベースファイルを手にしていることは、すでにコミュニティのデータのすべてを手に
// していることであるため、運用者はあらゆるスコープを許されます。これが、まだ誰も管理者で
// ないインスタンスで最初の管理者を立てられる理由です。
func OperatorActor() Actor {
	return Actor{isOperator: true}
}

// resolveCommunityPolicyは操作者のポリシーを構築します。管理系のUseCaseは
// すべてこの1つの関数を通るため、操作者に何が許されるかは、どこで問われても同じ形で
// 答えられます。
//
// 認可は書き込みトランザクションの開始前に取得したロールで判定し、トランザクション内では
// 再検証しません。書き込みが自身のトランザクションの中で読み直すのは、壊してはならない
// 状態、たとえばあるロールをまだ何人が持っているかです。
//
// 運用者はクエリを発行せずに解決します。運用者を記述する行は無く、運用者はデータベースの
// 中の割当ではなく、そのファイルを手にしていることによって認識されるためです。
func resolveCommunityPolicy(
	ctx context.Context,
	roleRepo *repository.RoleRepository,
	actor Actor,
) (*policy.CommunityPolicy, error) {
	if actor.isOperator {
		return policy.NewCommunityPolicy([]model.Scope{model.ScopeCommunityAdmin}), nil
	}

	roles, err := roleRepo.ListByUserID(ctx, actor.userID)
	if err != nil {
		return nil, fmt.Errorf("操作者のロールの取得に失敗: %w", err)
	}

	var scopes []model.Scope
	for _, role := range roles {
		scopes = append(scopes, role.Scopes...)
	}

	return policy.NewCommunityPolicy(scopes), nil
}
