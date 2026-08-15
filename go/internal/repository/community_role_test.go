package repository_test

import (
	"context"
	"testing"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/query"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
)

// newCommunityRoleRepos builds a CommunityRepository and a
// CommunityRoleRepository bound to the same test transaction. Both are needed
// because every role requires a community to belong to, and they must share one
// transaction for the role's foreign key to see the community.
//
// [Ja] newCommunityRoleRepos は同じテスト用トランザクションに束ねた
// CommunityRepository と CommunityRoleRepository を作る。ロールは必ず所属先の
// コミュニティを要するため両方が必要で、ロールの外部キーからコミュニティが見えるように
// 両者は 1 つのトランザクションを共有しなければならない。
func newCommunityRoleRepos(t *testing.T) (*repository.CommunityRepository, *repository.CommunityRoleRepository, context.Context) {
	t.Helper()
	db, tx := testutil.SetupTx(t)
	q := query.New(db)
	return repository.NewCommunityRepository(q).WithTx(tx),
		repository.NewCommunityRoleRepository(q).WithTx(tx),
		context.Background()
}

func TestCommunityRoleRepository_Create(t *testing.T) {
	t.Parallel()

	communityRepo, roleRepo, ctx := newCommunityRoleRepos(t)

	community, err := communityRepo.Create(ctx, repository.CreateCommunityInput{
		Name:       "ロール作成テストコミュニティ",
		Identifier: "create-community-role",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	role, err := roleRepo.Create(ctx, repository.CreateCommunityRoleInput{
		CommunityID: community.ID,
		Name:        "管理者",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if role.ID == (model.CommunityRoleID{}) {
		t.Error("Create() role.ID は DB 採番で空でないはず")
	}
	if role.CommunityID != community.ID {
		t.Errorf("role.CommunityID = %v, want %v", role.CommunityID, community.ID)
	}
	if role.Name != "管理者" {
		t.Errorf("role.Name = %q, want %q", role.Name, "管理者")
	}
	if role.CreatedAt.IsZero() {
		t.Error("role.CreatedAt は DB 既定値で設定されるはず")
	}
	if role.UpdatedAt.IsZero() {
		t.Error("role.UpdatedAt は DB 既定値で設定されるはず")
	}
}

// TestCommunityRoleRepository_CreateScopesNameUniquenessToCommunity verifies
// that the UNIQUE (community_id, name) constraint scopes role names to their
// community: another community may reuse a name, while the owning community may
// not. The duplicate insert comes last because the failed statement aborts the
// test transaction, leaving no room for further writes.
//
// [Ja] TestCommunityRoleRepository_CreateScopesNameUniquenessToCommunity は、
// UNIQUE (community_id, name) 制約がロール名の一意性をコミュニティ単位に限定することを
// 確認する。別のコミュニティは同じ名前を再利用できる一方、同じコミュニティではできない。
// 重複する INSERT を最後に置くのは、失敗したステートメントがテスト用トランザクションを
// abort させ、以降の書き込みができなくなるため。
func TestCommunityRoleRepository_CreateScopesNameUniquenessToCommunity(t *testing.T) {
	t.Parallel()

	communityRepo, roleRepo, ctx := newCommunityRoleRepos(t)

	community, err := communityRepo.Create(ctx, repository.CreateCommunityInput{
		Name:       "1 つ目のコミュニティ",
		Identifier: "duplicate-role-community",
	})
	if err != nil {
		t.Fatalf("1 つ目のコミュニティの Create() error = %v", err)
	}
	otherCommunity, err := communityRepo.Create(ctx, repository.CreateCommunityInput{
		Name:       "2 つ目のコミュニティ",
		Identifier: "duplicate-role-other-community",
	})
	if err != nil {
		t.Fatalf("2 つ目のコミュニティの Create() error = %v", err)
	}

	if _, err := roleRepo.Create(ctx, repository.CreateCommunityRoleInput{
		CommunityID: community.ID,
		Name:        "管理者",
	}); err != nil {
		t.Fatalf("1 回目の Create() error = %v", err)
	}

	t.Run("別のコミュニティなら同じロール名を使える", func(t *testing.T) {
		if _, err := roleRepo.Create(ctx, repository.CreateCommunityRoleInput{
			CommunityID: otherCommunity.ID,
			Name:        "管理者",
		}); err != nil {
			t.Errorf("別コミュニティでの同名ロールの Create() error = %v", err)
		}
	})

	t.Run("同じコミュニティで同じロール名は拒否される", func(t *testing.T) {
		if _, err := roleRepo.Create(ctx, repository.CreateCommunityRoleInput{
			CommunityID: community.ID,
			Name:        "管理者",
		}); err == nil {
			t.Error("同一コミュニティでロール名が重複する Create() はエラーになるはず")
		}
	})
}
