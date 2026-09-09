package repository_test

import (
	"context"
	"testing"

	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
)

// roleRepos bundles the two repositories a role is reached through, over one
// database the test owns. They come together because a role says nothing on its
// own: which roles a user holds and how many people hold a role are questions
// about the assignments, so a test of either repository creates rows through
// both.
//
// [Ja] roleRepos は、テストが所有する 1 つのデータベース上に、ロールに辿り着くための
// 2 つのリポジトリをまとめる。両者が揃うのは、ロールが単独では何も語らないためである。
// あるユーザーがどのロールを持つか、あるロールを何人が持つかはいずれも割当についての
// 問いであり、どちらのリポジトリを検証するテストも両方を通して行を作る。
type roleRepos struct {
	db       *database.DB
	role     *repository.RoleRepository
	userRole *repository.UserRoleRepository
}

// newRoleRepos builds the repositories over a fresh database.
//
// [Ja] newRoleRepos は新しいデータベース上にリポジトリ群を作る。
func newRoleRepos(t *testing.T) (*roleRepos, context.Context) {
	t.Helper()

	db := testutil.SetupDB(t)
	return &roleRepos{
		db:       db,
		role:     repository.NewRoleRepository(db),
		userRole: repository.NewUserRoleRepository(db),
	}, context.Background()
}

// findRole returns the role with the given name, failing the test when it is
// missing, for the tests whose subject is something other than the lookup.
//
// [Ja] findRole は指定した名前のロールを返し、存在しなければテストを失敗させる。
// ルックアップ自体を主題としないテストのためのものである。
func (r *roleRepos) findRole(t *testing.T, ctx context.Context, name model.RoleName) *model.Role {
	t.Helper()

	role, err := r.role.FindByName(ctx, name)
	if err != nil {
		t.Fatalf("FindByName() error = %v", err)
	}
	if role == nil {
		t.Fatalf("FindByName(%q) = nil, want a role", name)
	}

	return role
}

func TestRoleRepository_FindByName(t *testing.T) {
	t.Parallel()

	t.Run("組み込みの admin ロールを community:admin 付きで返す", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newRoleRepos(t)

		role := repos.findRole(t, ctx, model.RoleNameAdmin)

		if role.ID == 0 {
			t.Error("FindByName() role.ID は DB 採番で空でないはず")
		}
		if role.Name != model.RoleNameAdmin {
			t.Errorf("role.Name = %q, want %q", role.Name, model.RoleNameAdmin)
		}
		if len(role.Scopes) != 1 || role.Scopes[0] != model.ScopeCommunityAdmin {
			t.Errorf("role.Scopes = %v, want [%q]", role.Scopes, model.ScopeCommunityAdmin)
		}
		if role.CreatedAt.IsZero() {
			t.Error("role.CreatedAt は DB 既定値で設定されるはず")
		}
		if role.UpdatedAt.IsZero() {
			t.Error("role.UpdatedAt は DB 既定値で設定されるはず")
		}
	})

	t.Run("どのロールも持たない名前には nil を返す", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newRoleRepos(t)

		role, err := repos.role.FindByName(ctx, model.RoleName("moderator"))
		if err != nil {
			t.Fatalf("FindByName() error = %v", err)
		}
		if role != nil {
			t.Errorf("FindByName() = %v, want nil", role)
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
			t.Fatalf("role.Scopes = %v, want %v", role.Scopes, want)
		}
		for i, w := range want {
			if role.Scopes[i] != w {
				t.Errorf("role.Scopes[%d] = %q, want %q", i, role.Scopes[i], w)
			}
		}
	})

	t.Run("スコープが空のロールは空のスコープを持つ", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newRoleRepos(t)
		testutil.NewRoleBuilder(t, repos.db).WithName("guest").Build()

		role := repos.findRole(t, ctx, "guest")

		if len(role.Scopes) != 0 {
			t.Errorf("role.Scopes = %v, want 空", role.Scopes)
		}
	})

	t.Run("scopes が文字列の配列でなければエラーを返す", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newRoleRepos(t)
		// The column's check constraint admits any JSON array, so a role whose
		// scopes are not strings is a row the database accepts and the
		// repository cannot turn into a model.
		//
		// [Ja] 列のチェック制約は JSON 配列なら何でも許すため、スコープが文字列でない
		// ロールは、データベースが受け入れる一方でリポジトリがモデルに変換できない行である。
		testutil.NewRoleBuilder(t, repos.db).WithName("broken").WithRawScopes(`[1, 2]`).Build()

		_, err := repos.role.FindByName(ctx, "broken")
		if err == nil {
			t.Fatal("FindByName() error = nil, want a decode error")
		}
	})
}

func TestRoleRepository_ListByUserID(t *testing.T) {
	t.Parallel()

	t.Run("そのユーザーが持つロールだけを id 順に返す", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newRoleRepos(t)
		testutil.NewRoleBuilder(t, repos.db).
			WithName("moderator").
			WithScopes([]model.Scope{model.ScopeUserRead}).
			Build()

		holder := testutil.NewUserBuilder(t, repos.db).Build()
		other := testutil.NewUserBuilder(t, repos.db).Build()

		// The moderator role is assigned first so that a result echoing the
		// order the assignments were written cannot pass for id order.
		//
		// [Ja] 割当が書かれた順をそのまま返す結果が id 順として通らないよう、moderator を
		// 先に割り当てている。
		testutil.NewUserRoleBuilder(t, repos.db).WithUserID(holder).WithRoleName("moderator").Build()
		testutil.NewUserRoleBuilder(t, repos.db).WithUserID(holder).Build()
		testutil.NewUserRoleBuilder(t, repos.db).WithUserID(other).Build()

		roles, err := repos.role.ListByUserID(ctx, holder)
		if err != nil {
			t.Fatalf("ListByUserID() error = %v", err)
		}

		want := []model.RoleName{model.RoleNameAdmin, "moderator"}
		if len(roles) != len(want) {
			t.Fatalf("len(ListByUserID()) = %d, want %d", len(roles), len(want))
		}
		for i, w := range want {
			if roles[i].Name != w {
				t.Errorf("ListByUserID()[%d].Name = %q, want %q", i, roles[i].Name, w)
			}
		}
	})

	t.Run("ロールを持たないユーザーには空を返す", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newRoleRepos(t)
		userID := testutil.NewUserBuilder(t, repos.db).Build()

		roles, err := repos.role.ListByUserID(ctx, userID)
		if err != nil {
			t.Fatalf("ListByUserID() error = %v", err)
		}
		if len(roles) != 0 {
			t.Errorf("len(ListByUserID()) = %d, want 0", len(roles))
		}
	})

	t.Run("返すロールはスコープを伴う", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newRoleRepos(t)
		userID := testutil.NewUserBuilder(t, repos.db).Build()
		testutil.NewUserRoleBuilder(t, repos.db).WithUserID(userID).Build()

		roles, err := repos.role.ListByUserID(ctx, userID)
		if err != nil {
			t.Fatalf("ListByUserID() error = %v", err)
		}
		if len(roles) != 1 {
			t.Fatalf("len(ListByUserID()) = %d, want 1", len(roles))
		}
		if len(roles[0].Scopes) != 1 || roles[0].Scopes[0] != model.ScopeCommunityAdmin {
			t.Errorf("ListByUserID()[0].Scopes = %v, want [%q]", roles[0].Scopes, model.ScopeCommunityAdmin)
		}
	})
}

func TestRoleRepository_ListByUserIDs(t *testing.T) {
	t.Parallel()

	t.Run("利用者ごとにまとめ、ロールの id 順で返す", func(t *testing.T) {
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
			t.Fatalf("ListByUserIDs() error = %v", err)
		}

		wantIDs := []model.RoleID{admin.ID, readerID}
		if admin.ID > readerID {
			wantIDs = []model.RoleID{readerID, admin.ID}
		}
		gotIDs := roleIDs(rolesByUserID[both])
		if len(gotIDs) != len(wantIDs) || gotIDs[0] != wantIDs[0] || gotIDs[1] != wantIDs[1] {
			t.Errorf("両方のロールを持つ利用者のロール = %v, want %v", gotIDs, wantIDs)
		}

		if got := roleIDs(rolesByUserID[onlyReader]); len(got) != 1 || got[0] != readerID {
			t.Errorf("1 つだけ持つ利用者のロール = %v, want [%v]", got, readerID)
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
			t.Fatalf("ListByUserIDs() error = %v", err)
		}

		roles := rolesByUserID[userID]
		if len(roles) != 1 {
			t.Fatalf("len(roles) = %d, want 1", len(roles))
		}
		if len(roles[0].Scopes) != 1 || roles[0].Scopes[0] != model.ScopeCommunityAdmin {
			t.Errorf("roles[0].Scopes = %v, want [%q]", roles[0].Scopes, model.ScopeCommunityAdmin)
		}
	})

	// A closed reader turns any query into an error, so the empty map coming
	// back is the whole of the evidence that none was issued.
	//
	// [Ja] 閉じた読み取り接続はどのクエリもエラーに変えるため、空のマップが返ること自体が、
	// クエリを 1 つも発行しなかったことの証拠になる。
	t.Run("空の id 一覧ではクエリを発行しない", func(t *testing.T) {
		t.Parallel()

		repos, ctx := newRoleRepos(t)
		if err := repos.db.Reader.Close(); err != nil {
			t.Fatalf("Reader の Close() error = %v", err)
		}

		rolesByUserID, err := repos.role.ListByUserIDs(ctx, nil)
		if err != nil {
			t.Fatalf("ListByUserIDs() error = %v", err)
		}
		if len(rolesByUserID) != 0 {
			t.Errorf("ListByUserIDs(nil) = %v, want an empty map", rolesByUserID)
		}
	})
}

// roleIDs returns the ids of the roles in the order they were listed.
//
// [Ja] roleIDs は、並んだロールの id をその順序のまま返す。
func roleIDs(roles []*model.Role) []model.RoleID {
	ids := make([]model.RoleID, len(roles))
	for i, role := range roles {
		ids[i] = role.ID
	}
	return ids
}
