package usecase_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/query"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/validator"
)

// newCreateCommunityUsecase wires the usecase over the shared pool (not a rolled-
// back transaction) because CreateCommunityUsecase opens its own transaction
// internally; an outer transaction's seed rows would be invisible to that inner
// transaction. It returns the repositories a test needs to seed the creator and
// to look the created community up again. Tests use unique identifiers so
// committed rows do not collide (the test database is reset by make test).
//
// [Ja] newCreateCommunityUsecase は共有プール (ロールバックされるトランザクションでは
// なく) で UseCase を組み立てます。CreateCommunityUsecase は内部で自前のトランザクションを
// 開くため、外側トランザクションで仕込んだ行はその内側トランザクションから見えないから
// です。テストが作成者を仕込み、作成されたコミュニティを引き直せるようリポジトリも返し
// ます。テストはユニークな識別子を使い、コミット済みの行が衝突しないようにします
// (テスト DB は make test がリセットする)。
func newCreateCommunityUsecase(t *testing.T) (*usecase.CreateCommunityUsecase, *repository.UserRepository, *repository.CommunityRepository) {
	t.Helper()

	db := testutil.GetTestDB()
	queries := query.New(db)
	userRepo := repository.NewUserRepository(queries)
	communityRepo := repository.NewCommunityRepository(queries)
	communityRoleRepo := repository.NewCommunityRoleRepository(queries)
	communityMemberRepo := repository.NewCommunityMemberRepository(queries)
	communityMemberRoleRepo := repository.NewCommunityMemberRoleRepository(queries)

	uc := usecase.NewCreateCommunityUsecase(
		db,
		validator.NewCommunityCreateValidator(communityRepo),
		communityRepo,
		communityRoleRepo,
		communityMemberRepo,
		communityMemberRoleRepo,
	)
	return uc, userRepo, communityRepo
}

// seedCommunityCreator creates a committed user to found a community, returning
// it so a test can drive creation as that user.
//
// [Ja] seedCommunityCreator はコミュニティを作成するコミット済みユーザーを作成し、
// テストがそのユーザーとして作成を駆動できるよう返す。
func seedCommunityCreator(t *testing.T, ctx context.Context, userRepo *repository.UserRepository) *model.User {
	t.Helper()

	user, err := userRepo.Create(ctx, repository.CreateUserInput{
		Email:    fmt.Sprintf("community-%s@example.com", uuid.NewString()),
		Atname:   testutil.UniqueAtname(),
		Locale:   "ja",
		TimeZone: "Asia/Tokyo",
	})
	if err != nil {
		t.Fatalf("ユーザーの作成に失敗: %v", err)
	}
	return user
}

// uniqueCommunityIdentifier returns a random identifier that fits the validator's
// format (ASCII letters/digits/hyphen, 20 chars max), so committed rows from
// parallel tests do not collide on the communities.identifier UNIQUE constraint.
//
// [Ja] uniqueCommunityIdentifier はバリデーターの形式 (ASCII 英数字 / ハイフン・最大
// 20 文字) に収まるランダムな識別子を返す。並行テストのコミット済み行が
// communities.identifier の UNIQUE 制約で衝突しないようにするためのもの。
func uniqueCommunityIdentifier(prefix string) string {
	return prefix + "-" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
}

// TestCreateCommunityUsecase_Execute_Success verifies that a valid submission
// creates the community together with the administrator role, the creator's
// membership, and the assignment that gives the creator that role.
//
// [Ja] TestCreateCommunityUsecase_Execute_Success は、有効な送信がコミュニティを
// 管理者ロール・作成者のメンバーシップ・作成者へそのロールを与える割当と併せて作成する
// ことを検証します。
func TestCreateCommunityUsecase_Execute_Success(t *testing.T) {
	t.Parallel()

	uc, userRepo, communityRepo := newCreateCommunityUsecase(t)
	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)

	creator := seedCommunityCreator(t, ctx, userRepo)
	identifier := uniqueCommunityIdentifier("ok")

	out, err := uc.Execute(ctx, usecase.CreateCommunityInput{
		UserID:     creator.ID,
		Name:       "作成テストコミュニティ",
		Identifier: identifier,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if out == nil || out.Community == nil {
		t.Fatal("Execute() output / Community = nil")
	}
	if out.Community.Name != "作成テストコミュニティ" {
		t.Errorf("out.Community.Name = %q, want %q", out.Community.Name, "作成テストコミュニティ")
	}
	if out.Community.Identifier != identifier {
		t.Errorf("out.Community.Identifier = %q, want %q", out.Community.Identifier, identifier)
	}

	community, err := communityRepo.FindByIdentifier(ctx, identifier)
	if err != nil {
		t.Fatalf("FindByIdentifier() error = %v", err)
	}
	if community == nil {
		t.Fatal("作成したコミュニティを identifier で引けない")
	}

	// The creator holds a role named 管理者 in the community they founded: the
	// assignment, its member, and its role all belong to that community.
	//
	// [Ja] 作成者は自分が作ったコミュニティで 管理者 という名前のロールを持つ。割当・
	// そのメンバー・そのロールはいずれもそのコミュニティに属している。
	var roleName string
	if err := testutil.GetTestDB().QueryRow(ctx, `
		SELECT community_roles.name
		FROM community_member_roles
		JOIN community_members ON community_members.id = community_member_roles.community_member_id
		JOIN community_roles ON community_roles.id = community_member_roles.community_role_id
		WHERE community_member_roles.community_id = $1
		  AND community_members.community_id = $1
		  AND community_roles.community_id = $1
		  AND community_members.user_id = $2
	`, uuid.UUID(community.ID), uuid.UUID(creator.ID)).Scan(&roleName); err != nil {
		t.Fatalf("作成者へのロール割当の取得に失敗: %v", err)
	}
	if roleName != "管理者" {
		t.Errorf("作成者へ割り当てられたロール名 = %q, want %q", roleName, "管理者")
	}
}

// TestCreateCommunityUsecase_Execute_InvalidInput verifies that a submission the
// validator rejects (here, a reserved identifier) returns a ValidationError on
// the identifier field and creates no community.
//
// [Ja] TestCreateCommunityUsecase_Execute_InvalidInput は、バリデーターが却下する送信
// (ここでは予約語の識別子) が identifier フィールドの ValidationError を返し、コミュニティを
// 作成しないことを検証します。
func TestCreateCommunityUsecase_Execute_InvalidInput(t *testing.T) {
	t.Parallel()

	uc, userRepo, communityRepo := newCreateCommunityUsecase(t)
	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)

	creator := seedCommunityCreator(t, ctx, userRepo)

	out, err := uc.Execute(ctx, usecase.CreateCommunityInput{
		UserID:     creator.ID,
		Name:       "予約語コミュニティ",
		Identifier: "www",
	})
	if out != nil {
		t.Errorf("Execute() output = %v, want nil", out)
	}
	ve := model.AsValidationError(err)
	if ve == nil {
		t.Fatalf("Execute() error = %v, want *model.ValidationError", err)
	}
	if !ve.HasFieldError("identifier") {
		t.Errorf("identifier フィールドのエラーが無い: %+v", ve.Fields)
	}

	community, err := communityRepo.FindByIdentifier(ctx, "www")
	if err != nil {
		t.Fatalf("FindByIdentifier() error = %v", err)
	}
	if community != nil {
		t.Error("バリデーションに失敗した場合はコミュニティを作成すべきでない")
	}
}

// TestCreateCommunityUsecase_Execute_RollsBackOnMembershipFailure verifies the
// transaction boundary: when a later write in the transaction fails (here the
// membership, because the creator does not exist), the community created earlier
// in the same transaction is rolled back rather than left behind without an
// administrator.
//
// [Ja] TestCreateCommunityUsecase_Execute_RollsBackOnMembershipFailure は
// トランザクション境界を検証します。トランザクション内の後続の書き込みが失敗したとき
// (ここでは作成者が存在しないためメンバーシップが失敗)、同じトランザクションで先に作成された
// コミュニティは、管理者の居ないまま残らずロールバックされます。
func TestCreateCommunityUsecase_Execute_RollsBackOnMembershipFailure(t *testing.T) {
	t.Parallel()

	uc, _, communityRepo := newCreateCommunityUsecase(t)
	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)

	identifier := uniqueCommunityIdentifier("rb")

	out, err := uc.Execute(ctx, usecase.CreateCommunityInput{
		UserID:     model.UserID(uuid.New()),
		Name:       "ロールバックテストコミュニティ",
		Identifier: identifier,
	})
	if out != nil {
		t.Errorf("Execute() output = %v, want nil", out)
	}
	if err == nil {
		t.Fatal("Execute() error = nil, 存在しないユーザーでは失敗するはず")
	}

	community, err := communityRepo.FindByIdentifier(ctx, identifier)
	if err != nil {
		t.Fatalf("FindByIdentifier() error = %v", err)
	}
	if community != nil {
		t.Error("メンバーシップの作成に失敗した場合はコミュニティもロールバックされるはず")
	}
}
