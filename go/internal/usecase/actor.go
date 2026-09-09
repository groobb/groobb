package usecase

import (
	"context"
	"fmt"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/policy"
	"github.com/groobb/groobb/go/internal/repository"
)

// Actor is who an administrative UseCase acts for. It is either a signed-in user,
// whose permission is the scopes of the roles they hold, or the operator running
// a groobb subcommand against the database file, who is admitted to everything.
//
// Being the operator is stated by calling OperatorActor rather than by leaving a
// user id out: a value that arrives without an id is a user nobody is, holding no
// role and admitted to nothing, so forgetting to pass the signed-in user refuses
// the operation instead of granting all of them.
//
// [Ja] Actor は、管理系の UseCase が誰のために動くかを表します。サインイン済みの利用者
// (許されることはその人が持つロールのスコープ) か、データベースファイルに対して groobb の
// サブコマンドを実行する運用者 (すべてを許される) のどちらかです。
//
// 運用者であることは、利用者 id を省くことではなく OperatorActor を呼ぶことで述べます。
// id を伴わずに届いた値は、誰でもない利用者、すなわちロールを 1 つも持たず何も許されない
// 利用者です。そのため、サインイン済みの利用者を渡し忘れたときに起こるのは、すべての操作が
// 許されることではなく、その操作が拒まれることです。
type Actor struct {
	userID     model.UserID
	isOperator bool
}

// UserActor returns the actor for a signed-in user. The id comes from the
// authenticated session, never from a form or a URL, so that who acts is decided
// by who signed in.
//
// [Ja] UserActor は、サインイン済みの利用者を表す操作者を返します。id は認証済みの
// セッションから来るものであり、フォームや URL から来るものではありません。誰が操作するか
// を、誰がサインインしたかが決めるようにするためです。
func UserActor(userID model.UserID) Actor {
	return Actor{userID: userID}
}

// OperatorActor returns the actor for whoever runs a groobb subcommand. Holding
// the database file is already the whole of the community's data, so the operator
// is admitted to every scope; this is what lets the first administrator be
// appointed on an instance where nobody is one yet.
//
// [Ja] OperatorActor は、groobb のサブコマンドを実行する人を表す操作者を返します。
// データベースファイルを手にしていることは、すでにコミュニティのデータのすべてを手に
// していることであるため、運用者はあらゆるスコープを許されます。これが、まだ誰も管理者で
// ないインスタンスで最初の管理者を立てられる理由です。
func OperatorActor() Actor {
	return Actor{isOperator: true}
}

// resolveCommunityPolicy builds the policy for the actor. Every administrative
// UseCase goes through this one function, so that what an actor is admitted to is
// answered the same way wherever it is asked.
//
// The roles are read outside the transaction that writes: permission is about who
// the actor is, and re-reading it under the write lock would not make the answer
// any more current than the request that carried it. What a write does re-read
// inside its transaction is the state it must not break, such as how many people
// still hold a role.
//
// The operator is resolved without a query, because no row describes them: they
// are recognized by holding the database file rather than by an assignment in it.
//
// [Ja] resolveCommunityPolicy は操作者のポリシーを構築します。管理系の UseCase は
// すべてこの 1 つの関数を通るため、操作者に何が許されるかは、どこで問われても同じ形で
// 答えられます。
//
// ロールは書き込むトランザクションの外で読みます。権限は操作者が誰であるかについての
// ものであり、書き込みロックの下で読み直しても、その答えは要求が運んできたものより
// 新しくなりません。書き込みが自身のトランザクションの中で読み直すのは、壊してはならない
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
