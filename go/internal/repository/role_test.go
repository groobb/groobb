package repository_test

import (
	"context"
	"testing"

	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
)

// roleReposは、テストが所有する1つのデータベース上に、ロールに辿り着くための
// 2つのリポジトリをまとめる。両者が揃うのは、ロールが単独では何も語らないためである。
// あるユーザーがどのロールを持つか、あるロールを何人が持つかはいずれも割当についての
// 問いであり、どちらのリポジトリを検証するテストも両方を通して行を作る。
type roleRepos struct {
	db       *database.DB
	role     *repository.RoleRepository
	userRole *repository.UserRoleRepository
}

// newRoleReposは新しいデータベース上にリポジトリ群を作る。
func newRoleRepos(t *testing.T) (*roleRepos, context.Context) {
	t.Helper()

	db := testutil.SetupDB(t)
	return &roleRepos{
		db:       db,
		role:     repository.NewRoleRepository(db),
		userRole: repository.NewUserRoleRepository(db),
	}, context.Background()
}

// findRoleは指定した名前のロールを返し、存在しなければテストを失敗させる。
// ルックアップ自体を主題としないテストのためのものである。
func (r *roleRepos) findRole(t *testing.T, ctx context.Context, name model.RoleName) *model.Role {
	t.Helper()

	role, err := r.role.FindByName(ctx, name)
	if err != nil {
		t.Fatalf("FindByName()のエラー = %v", err)
	}
	if role == nil {
		t.Fatalf("FindByName(%q) = nil、期待値はロール", name)
	}

	return role
}

func TestRoleRepository_FindByName(t *testing.T) {
	t.Parallel()

	t.Run("組み込みのadminロールをcommunity:admin付きで返す", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newRoleRepos(t)

		role := repos.findRole(t, ctx, model.RoleNameAdmin)

		if role.ID == 0 {
			t.Error("FindByName() role.IDはDB採番で空でないはず")
		}
		if role.Name != model.RoleNameAdmin {
			t.Errorf("role.Name = %q、期待値 = %q", role.Name, model.RoleNameAdmin)
		}
		if len(role.Scopes) != 1 || role.Scopes[0] != model.ScopeCommunityAdmin {
			t.Errorf("role.Scopes = %v、期待値 = [%q]", role.Scopes, model.ScopeCommunityAdmin)
		}
		if role.CreatedAt.IsZero() {
			t.Error("role.CreatedAtはDB既定値で設定されるはず")
		}
		if role.UpdatedAt.IsZero() {
			t.Error("role.UpdatedAtはDB既定値で設定されるはず")
		}
	})

	t.Run("どのロールも持たない名前にはnilを返す", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newRoleRepos(t)

		role, err := repos.role.FindByName(ctx, model.RoleName("moderator"))
		if err != nil {
			t.Fatalf("FindByName()のエラー = %v", err)
		}
		if role != nil {
			t.Errorf("FindByName() = %v、期待値 = nil", role)
		}
	})

	t.Run("語彙に無いスコープも落とさずに返す", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newRoleRepos(t)
		testutil.NewRoleBuilder(t, repos.db).
			WithName("moderator").
			WithScopes([]model.Scope{model.ScopeUserRead, model.Scope("thread:lock")}).
			Build()

		role := repos.findRole(t, ctx, "moderator")

		want := []model.Scope{model.ScopeUserRead, model.Scope("thread:lock")}
		if len(role.Scopes) != len(want) {
			t.Fatalf("role.Scopes = %v、期待値 = %v", role.Scopes, want)
		}
		for i, w := range want {
			if role.Scopes[i] != w {
				t.Errorf("role.Scopes[%d] = %q、期待値 = %q", i, role.Scopes[i], w)
			}
		}
	})

	t.Run("スコープが空のロールは空のスコープを持つ", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newRoleRepos(t)
		testutil.NewRoleBuilder(t, repos.db).WithName("guest").Build()

		role := repos.findRole(t, ctx, "guest")

		if len(role.Scopes) != 0 {
			t.Errorf("role.Scopes = %v、期待値は空", role.Scopes)
		}
	})

	t.Run("scopesが文字列の配列でなければエラーを返す", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newRoleRepos(t)
		// 列のチェック制約はJSON配列なら何でも許すため、スコープが文字列でない
		// ロールは、データベースが受け入れる一方でリポジトリがモデルに変換できない行である。
		testutil.NewRoleBuilder(t, repos.db).WithName("broken").WithRawScopes(`[1, 2]`).Build()

		_, err := repos.role.FindByName(ctx, "broken")
		if err == nil {
			t.Fatal("FindByName()のエラー = nil、期待値はデコードエラー")
		}
	})
}

func TestRoleRepository_ListByUserID(t *testing.T) {
	t.Parallel()

	t.Run("そのユーザーが持つロールだけをid順に返す", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newRoleRepos(t)
		testutil.NewRoleBuilder(t, repos.db).
			WithName("moderator").
			WithScopes([]model.Scope{model.ScopeUserRead}).
			Build()

		holder := testutil.NewUserBuilder(t, repos.db).Build()
		other := testutil.NewUserBuilder(t, repos.db).Build()

		// 割当が書かれた順をそのまま返す結果がid順として通らないよう、moderatorを
		// 先に割り当てている。
		testutil.NewUserRoleBuilder(t, repos.db).WithUserID(holder).WithRoleName("moderator").Build()
		testutil.NewUserRoleBuilder(t, repos.db).WithUserID(holder).Build()
		testutil.NewUserRoleBuilder(t, repos.db).WithUserID(other).Build()

		roles, err := repos.role.ListByUserID(ctx, holder)
		if err != nil {
			t.Fatalf("ListByUserID()のエラー = %v", err)
		}

		want := []model.RoleName{model.RoleNameAdmin, "moderator"}
		if len(roles) != len(want) {
			t.Fatalf("len(ListByUserID()) = %d、期待値 = %d", len(roles), len(want))
		}
		for i, w := range want {
			if roles[i].Name != w {
				t.Errorf("ListByUserID()[%d].Name = %q、期待値 = %q", i, roles[i].Name, w)
			}
		}
	})

	t.Run("ロールを持たないユーザーには空を返す", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newRoleRepos(t)
		userID := testutil.NewUserBuilder(t, repos.db).Build()

		roles, err := repos.role.ListByUserID(ctx, userID)
		if err != nil {
			t.Fatalf("ListByUserID()のエラー = %v", err)
		}
		if len(roles) != 0 {
			t.Errorf("len(ListByUserID()) = %d、期待値 = 0", len(roles))
		}
	})

	t.Run("返すロールはスコープを伴う", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newRoleRepos(t)
		userID := testutil.NewUserBuilder(t, repos.db).Build()
		testutil.NewUserRoleBuilder(t, repos.db).WithUserID(userID).Build()

		roles, err := repos.role.ListByUserID(ctx, userID)
		if err != nil {
			t.Fatalf("ListByUserID()のエラー = %v", err)
		}
		if len(roles) != 1 {
			t.Fatalf("len(ListByUserID()) = %d、期待値 = 1", len(roles))
		}
		if len(roles[0].Scopes) != 1 || roles[0].Scopes[0] != model.ScopeCommunityAdmin {
			t.Errorf("ListByUserID()[0].Scopes = %v、期待値 = [%q]", roles[0].Scopes, model.ScopeCommunityAdmin)
		}
	})
}

func TestRoleRepository_ListByUserIDs(t *testing.T) {
	t.Parallel()

	t.Run("利用者ごとにまとめ、ロールのid順で返す", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newRoleRepos(t)
		admin := repos.findRole(t, ctx, model.RoleNameAdmin)
		const readerName = model.RoleName("user_reader")
		readerID := testutil.NewRoleBuilder(t, repos.db).
			WithName(readerName).
			WithScopes([]model.Scope{model.ScopeUserRead}).
			Build()

		both := testutil.NewUserBuilder(t, repos.db).Build()
		onlyReader := testutil.NewUserBuilder(t, repos.db).Build()
		none := testutil.NewUserBuilder(t, repos.db).Build()
		testutil.NewUserRoleBuilder(t, repos.db).WithUserID(both).Build()
		testutil.NewUserRoleBuilder(t, repos.db).WithUserID(both).WithRoleName(readerName).Build()
		testutil.NewUserRoleBuilder(t, repos.db).WithUserID(onlyReader).WithRoleName(readerName).Build()

		rolesByUserID, err := repos.role.ListByUserIDs(ctx, []model.UserID{both, onlyReader, none})
		if err != nil {
			t.Fatalf("ListByUserIDs()のエラー = %v", err)
		}

		wantIDs := []model.RoleID{admin.ID, readerID}
		if admin.ID > readerID {
			wantIDs = []model.RoleID{readerID, admin.ID}
		}
		gotIDs := roleIDs(rolesByUserID[both])
		if len(gotIDs) != len(wantIDs) || gotIDs[0] != wantIDs[0] || gotIDs[1] != wantIDs[1] {
			t.Errorf("両方のロールを持つ利用者のロール = %v、期待値 = %v", gotIDs, wantIDs)
		}

		if got := roleIDs(rolesByUserID[onlyReader]); len(got) != 1 || got[0] != readerID {
			t.Errorf("1つだけ持つ利用者のロール = %v、期待値 = [%v]", got, readerID)
		}

		if _, ok := rolesByUserID[none]; ok {
			t.Errorf("ロールを持たない利用者はマップに現れないはず: %v", rolesByUserID[none])
		}
	})

	t.Run("スコープを復元して返す", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newRoleRepos(t)
		userID := testutil.NewUserBuilder(t, repos.db).Build()
		testutil.NewUserRoleBuilder(t, repos.db).WithUserID(userID).Build()

		rolesByUserID, err := repos.role.ListByUserIDs(ctx, []model.UserID{userID})
		if err != nil {
			t.Fatalf("ListByUserIDs()のエラー = %v", err)
		}

		roles := rolesByUserID[userID]
		if len(roles) != 1 {
			t.Fatalf("len(roles) = %d、期待値 = 1", len(roles))
		}
		if len(roles[0].Scopes) != 1 || roles[0].Scopes[0] != model.ScopeCommunityAdmin {
			t.Errorf("roles[0].Scopes = %v、期待値 = [%q]", roles[0].Scopes, model.ScopeCommunityAdmin)
		}
	})

	// 閉じた読み取り接続はどのクエリもエラーに変えるため、空のマップが返ること自体が、
	// クエリを1つも発行しなかったことの証拠になる。
	t.Run("空のid一覧ではクエリを発行しない", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newRoleRepos(t)
		if err := repos.db.Reader.Close(); err != nil {
			t.Fatalf("ReaderのClose()のエラー = %v", err)
		}

		rolesByUserID, err := repos.role.ListByUserIDs(ctx, nil)
		if err != nil {
			t.Fatalf("ListByUserIDs()のエラー = %v", err)
		}
		if len(rolesByUserID) != 0 {
			t.Errorf("ListByUserIDs(nil) = %v、期待値は空のmap", rolesByUserID)
		}
	})
}

// roleIDsは、並んだロールのidをその順序のまま返す。
func roleIDs(roles []*model.Role) []model.RoleID {
	ids := make([]model.RoleID, len(roles))
	for i, role := range roles {
		ids[i] = role.ID
	}
	return ids
}
