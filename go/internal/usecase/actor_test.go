package usecase

import (
	"context"
	"testing"

	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
)

// The test lives in the usecase package rather than in usecase_test because
// resolveCommunityPolicy is unexported: it is the one place administrative
// UseCases turn an actor into permission, and nothing outside this package may
// build a policy of its own.
//
// [Ja] 本テストが usecase_test ではなく usecase パッケージに置かれているのは、
// resolveCommunityPolicy が非公開であるためです。この関数は管理系の UseCase が操作者を
// 権限へ変える唯一の場所であり、このパッケージの外が自前でポリシーを組み立てることは
// ありません。

// setupActorTest gives the test a database of its own together with the role
// repository the resolution reads through.
//
// [Ja] setupActorTest は、テストに自身のデータベースと、解決が読み取りに使うロールの
// リポジトリを与えます。
func setupActorTest(t *testing.T) (*database.DB, *repository.RoleRepository) {
	t.Helper()

	db := testutil.SetupDB(t)
	return db, repository.NewRoleRepository(db)
}

// createAndGrantRole creates a role carrying the given scopes and assigns it to
// the user. The role is created here because the only one a migrated database
// holds is the built-in admin, while this test is about what roles narrower than
// it admit.
//
// [Ja] createAndGrantRole は、渡したスコープを持つロールを作ってユーザーへ割り当てます。
// ここでロールを作るのは、マイグレーション済みのデータベースが持つロールが組み込みの
// admin だけである一方、本テストが問うのはそれより狭いロールが何を許すかであるためです。
func createAndGrantRole(t *testing.T, db *database.DB, userID model.UserID, name model.RoleName, scopes []model.Scope) {
	t.Helper()

	testutil.NewRoleBuilder(t, db).WithName(name).WithScopes(scopes).Build()
	testutil.NewUserRoleBuilder(t, db).WithUserID(userID).WithRoleName(name).Build()
}

func TestResolveCommunityPolicy(t *testing.T) {
	t.Parallel()

	t.Run("運用者はロールを読まずに全スコープを許される", func(t *testing.T) {
		t.Parallel()

		// The repository is nil so that the test fails loudly if the resolution
		// ever queries for the operator: no row describes them, and an instance
		// where nobody is an administrator yet must still be able to appoint one.
		//
		// [Ja] リポジトリを nil にしているのは、運用者に対して解決がクエリを発行した
		// 場合にテストがはっきり失敗するようにするためです。運用者を記述する行は無く、
		// まだ誰も管理者でないインスタンスでも最初の管理者を立てられなければなりません。
		p, err := resolveCommunityPolicy(context.Background(), nil, OperatorActor())
		if err != nil {
			t.Fatalf("resolveCommunityPolicy() error = %v", err)
		}

		if !p.CanAccessAdmin() || !p.CanListUsers() || !p.CanGrantUserRole() || !p.CanRevokeUserRole() {
			t.Errorf("運用者のポリシーが全スコープを許していない: %+v", p)
		}
	})

	t.Run("admin ロールを持つ利用者は全スコープを許される", func(t *testing.T) {
		t.Parallel()

		db, roleRepo := setupActorTest(t)
		userID := testutil.NewUserBuilder(t, db).Build()
		testutil.NewUserRoleBuilder(t, db).WithUserID(userID).Build()

		p, err := resolveCommunityPolicy(context.Background(), roleRepo, UserActor(userID))
		if err != nil {
			t.Fatalf("resolveCommunityPolicy() error = %v", err)
		}

		if !p.CanAccessAdmin() || !p.CanListUsers() || !p.CanGrantUserRole() || !p.CanRevokeUserRole() {
			t.Errorf("admin ロールを持つ利用者のポリシーが全スコープを許していない: %+v", p)
		}
	})

	t.Run("ロールを持たない利用者は何も許されない", func(t *testing.T) {
		t.Parallel()

		db, roleRepo := setupActorTest(t)
		userID := testutil.NewUserBuilder(t, db).Build()

		p, err := resolveCommunityPolicy(context.Background(), roleRepo, UserActor(userID))
		if err != nil {
			t.Fatalf("resolveCommunityPolicy() error = %v", err)
		}

		if p.CanAccessAdmin() || p.CanListUsers() || p.CanGrantUserRole() || p.CanRevokeUserRole() {
			t.Errorf("ロールを持たない利用者のポリシーが何かを許している: %+v", p)
		}
	})

	t.Run("複数のロールのスコープが合わさる", func(t *testing.T) {
		t.Parallel()

		db, roleRepo := setupActorTest(t)
		userID := testutil.NewUserBuilder(t, db).Build()
		createAndGrantRole(t, db, userID, "reader", []model.Scope{model.ScopeUserRead})
		createAndGrantRole(t, db, userID, "granter", []model.Scope{model.ScopeUserRoleWrite})

		p, err := resolveCommunityPolicy(context.Background(), roleRepo, UserActor(userID))
		if err != nil {
			t.Fatalf("resolveCommunityPolicy() error = %v", err)
		}

		if !p.CanListUsers() {
			t.Error("CanListUsers() = false, want true (user:read を持つロールを割り当てている)")
		}
		if !p.CanGrantUserRole() {
			t.Error("CanGrantUserRole() = false, want true (user_role:write を持つロールを割り当てている)")
		}
	})

	t.Run("語彙に無いスコープだけのロールは何も許さない", func(t *testing.T) {
		t.Parallel()

		db, roleRepo := setupActorTest(t)
		userID := testutil.NewUserBuilder(t, db).Build()
		createAndGrantRole(t, db, userID, "locker", []model.Scope{model.Scope("thread:lock")})

		p, err := resolveCommunityPolicy(context.Background(), roleRepo, UserActor(userID))
		if err != nil {
			t.Fatalf("resolveCommunityPolicy() error = %v", err)
		}

		if p.CanAccessAdmin() || p.CanListUsers() || p.CanGrantUserRole() || p.CanRevokeUserRole() {
			t.Errorf("語彙に無いスコープだけのポリシーが何かを許している: %+v", p)
		}
	})

	t.Run("操作者のロールを復元できなければポリシーを返さない", func(t *testing.T) {
		t.Parallel()

		db, roleRepo := setupActorTest(t)
		userID := testutil.NewUserBuilder(t, db).Build()
		// The role is written through the raw column because its elements are not
		// strings, which is what the repository fails to decode; the check
		// constraint admits it because it only asks for a JSON array.
		//
		// [Ja] このロールを生の列として書くのは、要素が文字列でないためです。リポジトリが
		// 復元に失敗するのがその形であり、チェック制約は JSON 配列であることしか尋ねない
		// ため、この行は保存できます。
		testutil.NewRoleBuilder(t, db).WithName("broken").WithRawScopes(`[1]`).Build()
		testutil.NewUserRoleBuilder(t, db).WithUserID(userID).WithRoleName("broken").Build()

		p, err := resolveCommunityPolicy(context.Background(), roleRepo, UserActor(userID))
		if err == nil {
			t.Fatal("resolveCommunityPolicy() error = nil, want a role decode error")
		}
		if p != nil {
			t.Errorf("resolveCommunityPolicy() policy = %+v, want nil", p)
		}
	})

	t.Run("サインインしていない値は何も許されない", func(t *testing.T) {
		t.Parallel()

		_, roleRepo := setupActorTest(t)

		// The zero value stands for an actor a caller forgot to fill in: it
		// carries a user id nobody has, so the operation is refused rather than
		// fully admitted.
		//
		// [Ja] ゼロ値は、呼び出し側が埋め忘れた操作者を表します。誰も持たない利用者 id を
		// 運ぶため、操作はすべて許されるのではなく拒まれます。
		p, err := resolveCommunityPolicy(context.Background(), roleRepo, Actor{})
		if err != nil {
			t.Fatalf("resolveCommunityPolicy() error = %v", err)
		}

		if p.CanAccessAdmin() || p.CanListUsers() || p.CanGrantUserRole() || p.CanRevokeUserRole() {
			t.Errorf("id を持たない操作者のポリシーが何かを許している: %+v", p)
		}
	})
}
