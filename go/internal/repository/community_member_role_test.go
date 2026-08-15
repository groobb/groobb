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

// communityMemberRoleRepos holds the repositories a role assignment touches,
// all bound to one test transaction, together with that transaction so a test
// can also insert the user a membership needs.
//
// [Ja] communityMemberRoleRepos はロール割当が触れるリポジトリを、すべて 1 つの
// テスト用トランザクションに束ねて保持する。メンバーシップに必要なユーザーもテストが
// 挿入できるよう、そのトランザクション自体も併せて持つ。
type communityMemberRoleRepos struct {
	tx         pgx.Tx
	community  *repository.CommunityRepository
	member     *repository.CommunityMemberRepository
	role       *repository.CommunityRoleRepository
	memberRole *repository.CommunityMemberRoleRepository
}

// newCommunityMemberRoleRepos builds the repositories bound to the test
// transaction, so writes roll back when the test finishes.
//
// [Ja] newCommunityMemberRoleRepos はテスト用トランザクションに束ねたリポジトリ群を
// 作る。テスト終了時に書き込みはロールバックされる。
func newCommunityMemberRoleRepos(t *testing.T) (*communityMemberRoleRepos, context.Context) {
	t.Helper()
	db, tx := testutil.SetupTx(t)
	q := query.New(db)
	return &communityMemberRoleRepos{
		tx:         tx,
		community:  repository.NewCommunityRepository(q).WithTx(tx),
		member:     repository.NewCommunityMemberRepository(q).WithTx(tx),
		role:       repository.NewCommunityRoleRepository(q).WithTx(tx),
		memberRole: repository.NewCommunityMemberRoleRepository(q).WithTx(tx),
	}, context.Background()
}

// communityFixture is a community that already has one member and one role,
// which is the state a role assignment starts from.
//
// [Ja] communityFixture はメンバー 1 人とロール 1 つを既に持つコミュニティで、
// ロール割当が出発点とする状態にあたる。
type communityFixture struct {
	community *model.Community
	member    *model.CommunityMember
	role      *model.CommunityRole
}

// buildCommunity creates a community under the given identifier with one member
// (backed by a fresh user) and one role.
//
// [Ja] buildCommunity は指定の identifier のコミュニティを、メンバー 1 人 (新規ユーザーに
// 紐づく) とロール 1 つを伴って作る。
func (r *communityMemberRoleRepos) buildCommunity(t *testing.T, ctx context.Context, identifier string) communityFixture {
	t.Helper()

	community, err := r.community.Create(ctx, repository.CreateCommunityInput{
		Name:       identifier,
		Identifier: identifier,
	})
	if err != nil {
		t.Fatalf("コミュニティの Create() error = %v", err)
	}

	member, err := r.member.Create(ctx, repository.CreateCommunityMemberInput{
		CommunityID: community.ID,
		UserID:      testutil.NewUserBuilder(t, r.tx).Build(),
	})
	if err != nil {
		t.Fatalf("メンバーの Create() error = %v", err)
	}

	role, err := r.role.Create(ctx, repository.CreateCommunityRoleInput{
		CommunityID: community.ID,
		Name:        "管理者",
	})
	if err != nil {
		t.Fatalf("ロールの Create() error = %v", err)
	}

	return communityFixture{community: community, member: member, role: role}
}

func TestCommunityMemberRoleRepository_Create(t *testing.T) {
	t.Parallel()

	repos, ctx := newCommunityMemberRoleRepos(t)
	fixture := repos.buildCommunity(t, ctx, "create-community-member-role")

	memberRole, err := repos.memberRole.Create(ctx, repository.CreateCommunityMemberRoleInput{
		CommunityID:       fixture.community.ID,
		CommunityMemberID: fixture.member.ID,
		CommunityRoleID:   fixture.role.ID,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if memberRole.ID == (model.CommunityMemberRoleID{}) {
		t.Error("Create() memberRole.ID は DB 採番で空でないはず")
	}
	if memberRole.CommunityID != fixture.community.ID {
		t.Errorf("memberRole.CommunityID = %v, want %v", memberRole.CommunityID, fixture.community.ID)
	}
	if memberRole.CommunityMemberID != fixture.member.ID {
		t.Errorf("memberRole.CommunityMemberID = %v, want %v", memberRole.CommunityMemberID, fixture.member.ID)
	}
	if memberRole.CommunityRoleID != fixture.role.ID {
		t.Errorf("memberRole.CommunityRoleID = %v, want %v", memberRole.CommunityRoleID, fixture.role.ID)
	}
	if memberRole.CreatedAt.IsZero() {
		t.Error("memberRole.CreatedAt は DB 既定値で設定されるはず")
	}
	if memberRole.UpdatedAt.IsZero() {
		t.Error("memberRole.UpdatedAt は DB 既定値で設定されるはず")
	}
}

// TestCommunityMemberRoleRepository_CreateRejectsRoleFromAnotherCommunity
// verifies the composite foreign key on the role side: an assignment naming one
// community may not point at a role defined by another. Because a failed
// statement aborts the test transaction, this case and the member-side one below
// each need a transaction of their own and so live in separate tests.
//
// [Ja] TestCommunityMemberRoleRepository_CreateRejectsRoleFromAnotherCommunity は
// ロール側の複合外部キーを検証する。あるコミュニティを指す割当が、別のコミュニティで
// 定義されたロールを指すことはできない。失敗したステートメントはテスト用トランザクションを
// abort させるため、本ケースと後続のメンバー側のケースはそれぞれ自分のトランザクションを
// 必要とし、別々のテストに分けている。
func TestCommunityMemberRoleRepository_CreateRejectsRoleFromAnotherCommunity(t *testing.T) {
	t.Parallel()

	repos, ctx := newCommunityMemberRoleRepos(t)
	fixture := repos.buildCommunity(t, ctx, "cross-role-community")
	other := repos.buildCommunity(t, ctx, "cross-role-other-community")

	if _, err := repos.memberRole.Create(ctx, repository.CreateCommunityMemberRoleInput{
		CommunityID:       fixture.community.ID,
		CommunityMemberID: fixture.member.ID,
		CommunityRoleID:   other.role.ID,
	}); err == nil {
		t.Error("別コミュニティのロールを割り当てる Create() はエラーになるはず")
	}
}

// TestCommunityMemberRoleRepository_CreateRejectsMemberFromAnotherCommunity
// verifies the composite foreign key on the member side: an assignment naming
// one community may not point at a member of another.
//
// [Ja] TestCommunityMemberRoleRepository_CreateRejectsMemberFromAnotherCommunity は
// メンバー側の複合外部キーを検証する。あるコミュニティを指す割当が、別のコミュニティの
// メンバーを指すことはできない。
func TestCommunityMemberRoleRepository_CreateRejectsMemberFromAnotherCommunity(t *testing.T) {
	t.Parallel()

	repos, ctx := newCommunityMemberRoleRepos(t)
	fixture := repos.buildCommunity(t, ctx, "cross-member-community")
	other := repos.buildCommunity(t, ctx, "cross-member-other-community")

	if _, err := repos.memberRole.Create(ctx, repository.CreateCommunityMemberRoleInput{
		CommunityID:       fixture.community.ID,
		CommunityMemberID: other.member.ID,
		CommunityRoleID:   fixture.role.ID,
	}); err == nil {
		t.Error("別コミュニティのメンバーへ割り当てる Create() はエラーになるはず")
	}
}

// TestCommunityMemberRoleRepository_CreateRejectsDuplicateAssignment verifies
// the UNIQUE (community_member_id, community_role_id) constraint: a member holds
// a given role once, not twice.
//
// [Ja] TestCommunityMemberRoleRepository_CreateRejectsDuplicateAssignment は
// UNIQUE (community_member_id, community_role_id) 制約を検証する。メンバーがある
// ロールを持つのは 1 度きりで、二重には持てない。
func TestCommunityMemberRoleRepository_CreateRejectsDuplicateAssignment(t *testing.T) {
	t.Parallel()

	repos, ctx := newCommunityMemberRoleRepos(t)
	fixture := repos.buildCommunity(t, ctx, "duplicate-member-role-community")

	input := repository.CreateCommunityMemberRoleInput{
		CommunityID:       fixture.community.ID,
		CommunityMemberID: fixture.member.ID,
		CommunityRoleID:   fixture.role.ID,
	}
	if _, err := repos.memberRole.Create(ctx, input); err != nil {
		t.Fatalf("1 回目の Create() error = %v", err)
	}

	if _, err := repos.memberRole.Create(ctx, input); err == nil {
		t.Error("同じメンバーへ同じロールを重ねて割り当てる Create() はエラーになるはず")
	}
}
