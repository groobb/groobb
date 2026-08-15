package repository_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/query"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
)

// newCommunityMemberRepos builds a CommunityRepository and a
// CommunityMemberRepository bound to the same test transaction, and hands back
// that transaction too: a membership also needs a user, which tests insert
// directly through UserBuilder. All three must share one transaction for the
// membership's foreign keys to see the community and the user.
//
// [Ja] newCommunityMemberRepos は同じテスト用トランザクションに束ねた
// CommunityRepository と CommunityMemberRepository を作り、そのトランザクション自体も
// 返す。メンバーシップにはユーザーも必要で、テストは UserBuilder 経由でそれを直接
// 挿入するため。メンバーシップの外部キーからコミュニティとユーザーの双方が見えるように、
// 3 者は 1 つのトランザクションを共有しなければならない。
func newCommunityMemberRepos(t *testing.T) (*repository.CommunityRepository, *repository.CommunityMemberRepository, pgx.Tx, context.Context) {
	t.Helper()
	db, tx := testutil.SetupTx(t)
	q := query.New(db)
	return repository.NewCommunityRepository(q).WithTx(tx),
		repository.NewCommunityMemberRepository(q).WithTx(tx),
		tx,
		context.Background()
}

func TestCommunityMemberRepository_Create(t *testing.T) {
	t.Parallel()

	communityRepo, memberRepo, tx, ctx := newCommunityMemberRepos(t)

	community, err := communityRepo.Create(ctx, repository.CreateCommunityInput{
		Name:       "メンバー作成テストコミュニティ",
		Identifier: "create-community-member",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	userID := testutil.NewUserBuilder(t, tx).Build()

	member, err := memberRepo.Create(ctx, repository.CreateCommunityMemberInput{
		CommunityID: community.ID,
		UserID:      userID,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if member.ID == (model.CommunityMemberID{}) {
		t.Error("Create() member.ID は DB 採番で空でないはず")
	}
	if member.CommunityID != community.ID {
		t.Errorf("member.CommunityID = %v, want %v", member.CommunityID, community.ID)
	}
	if member.UserID != userID {
		t.Errorf("member.UserID = %v, want %v", member.UserID, userID)
	}
	if member.CreatedAt.IsZero() {
		t.Error("member.CreatedAt は DB 既定値で設定されるはず")
	}
	if member.UpdatedAt.IsZero() {
		t.Error("member.UpdatedAt は DB 既定値で設定されるはず")
	}
}

// TestCommunityMemberRepository_CreateScopesMembershipUniquenessToCommunity
// verifies that the UNIQUE (community_id, user_id) constraint scopes membership
// to its community: the same user may join another community, but may not join
// the same one twice. The duplicate insert comes last because the failed
// statement aborts the test transaction, leaving no room for further writes.
//
// [Ja] TestCommunityMemberRepository_CreateScopesMembershipUniquenessToCommunity は、
// UNIQUE (community_id, user_id) 制約が所属の一意性をコミュニティ単位に限定することを
// 確認する。同じユーザーが別のコミュニティには参加できる一方、同じコミュニティへ二重には
// 参加できない。重複する INSERT を最後に置くのは、失敗したステートメントがテスト用
// トランザクションを abort させ、以降の書き込みができなくなるため。
func TestCommunityMemberRepository_CreateScopesMembershipUniquenessToCommunity(t *testing.T) {
	t.Parallel()

	communityRepo, memberRepo, tx, ctx := newCommunityMemberRepos(t)

	community, err := communityRepo.Create(ctx, repository.CreateCommunityInput{
		Name:       "1 つ目のコミュニティ",
		Identifier: "duplicate-member-community",
	})
	if err != nil {
		t.Fatalf("1 つ目のコミュニティの Create() error = %v", err)
	}
	otherCommunity, err := communityRepo.Create(ctx, repository.CreateCommunityInput{
		Name:       "2 つ目のコミュニティ",
		Identifier: "duplicate-member-other-community",
	})
	if err != nil {
		t.Fatalf("2 つ目のコミュニティの Create() error = %v", err)
	}
	userID := testutil.NewUserBuilder(t, tx).Build()

	if _, err := memberRepo.Create(ctx, repository.CreateCommunityMemberInput{
		CommunityID: community.ID,
		UserID:      userID,
	}); err != nil {
		t.Fatalf("1 回目の Create() error = %v", err)
	}

	t.Run("同じユーザーでも別のコミュニティには参加できる", func(t *testing.T) {
		if _, err := memberRepo.Create(ctx, repository.CreateCommunityMemberInput{
			CommunityID: otherCommunity.ID,
			UserID:      userID,
		}); err != nil {
			t.Errorf("別コミュニティへの参加の Create() error = %v", err)
		}
	})

	t.Run("同じコミュニティへの二重の参加は拒否される", func(t *testing.T) {
		if _, err := memberRepo.Create(ctx, repository.CreateCommunityMemberInput{
			CommunityID: community.ID,
			UserID:      userID,
		}); err == nil {
			t.Error("同一コミュニティへ二重に参加する Create() はエラーになるはず")
		}
	})
}
