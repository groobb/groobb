package usecase

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/validator"
)

// administratorRoleName is the name given to the role a new community grants its
// creator. A role carries only a name, so this constant is the whole of what
// marks the creator as the community's administrator until permission scopes
// exist. It is not localized per creator: a role name is data the community can
// later rename, not interface text.
//
// [Ja] administratorRoleName は新しいコミュニティが作成者へ与えるロールの名前です。
// ロールは名前だけを持つため、権限スコープができるまでは、作成者をそのコミュニティの
// 管理者と示すのはこの定数がすべてです。作成者ごとのローカライズは行いません。ロール名は
// 後からコミュニティ側で変更できるデータであり、インターフェースの文言ではないためです。
const administratorRoleName = "管理者"

// CreateCommunityUsecase orchestrates community creation: it validates the
// submitted name and identifier, then creates the community together with the
// administrator role, the creator's membership, and the assignment that joins
// them, in one transaction.
//
// [Ja] CreateCommunityUsecase はコミュニティ作成を統括します。送信された名前と URL
// 識別子を検証し、コミュニティを管理者ロール・作成者のメンバーシップ・両者を結ぶ割当と
// 併せて 1 トランザクションで作成します。
type CreateCommunityUsecase struct {
	db                      *pgxpool.Pool
	communityValidator      *validator.CommunityCreateValidator
	communityRepo           *repository.CommunityRepository
	communityRoleRepo       *repository.CommunityRoleRepository
	communityMemberRepo     *repository.CommunityMemberRepository
	communityMemberRoleRepo *repository.CommunityMemberRoleRepository
}

// NewCreateCommunityUsecase builds a CreateCommunityUsecase from the pool, the
// validator, and the repositories it persists through.
//
// [Ja] NewCreateCommunityUsecase はプール・validator・永続化に使うリポジトリから
// CreateCommunityUsecase を構築します。
func NewCreateCommunityUsecase(
	db *pgxpool.Pool,
	communityValidator *validator.CommunityCreateValidator,
	communityRepo *repository.CommunityRepository,
	communityRoleRepo *repository.CommunityRoleRepository,
	communityMemberRepo *repository.CommunityMemberRepository,
	communityMemberRoleRepo *repository.CommunityMemberRoleRepository,
) *CreateCommunityUsecase {
	return &CreateCommunityUsecase{
		db:                      db,
		communityValidator:      communityValidator,
		communityRepo:           communityRepo,
		communityRoleRepo:       communityRoleRepo,
		communityMemberRepo:     communityMemberRepo,
		communityMemberRoleRepo: communityMemberRoleRepo,
	}
}

// CreateCommunityInput is the input to Execute. UserID is the signed-in user
// founding the community, who becomes its first member and administrator; Name
// is the display name and Identifier the URL identifier taken from the form.
//
// [Ja] CreateCommunityInput は Execute の入力です。UserID はコミュニティを作成する
// サインイン済みユーザーで、最初のメンバーかつ管理者になります。Name は表示名、
// Identifier はフォームから受け取る URL 識別子です。
type CreateCommunityInput struct {
	UserID     model.UserID
	Name       string
	Identifier string
}

// CreateCommunityOutput carries the created community so the handler can send
// the creator to its page.
//
// [Ja] CreateCommunityOutput は作成されたコミュニティを運び、ハンドラーが作成者を
// その画面へ送れるようにします。
type CreateCommunityOutput struct {
	Community *model.Community
}

// Execute validates the submitted form and creates the community. Validation
// runs before the transaction so a rejected form never opens one.
//
// [Ja] Execute は送信されたフォームを検証し、コミュニティを作成します。バリデーションは
// トランザクションの前に行い、却下されるフォームがトランザクションを開かないようにします。
func (uc *CreateCommunityUsecase) Execute(ctx context.Context, input CreateCommunityInput) (*CreateCommunityOutput, error) {
	if err := uc.communityValidator.Validate(ctx, validator.CommunityCreateValidatorInput{
		Name:       input.Name,
		Identifier: input.Identifier,
	}); err != nil {
		return nil, err
	}

	return uc.createCommunity(ctx, input)
}

// createCommunity creates the community, its administrator role, the creator's
// membership, and the assignment of that role to that membership in one
// transaction, so a community never exists without an administrator to manage
// it (nor a role or membership without the community they belong to).
//
// [Ja] createCommunity はコミュニティ・その管理者ロール・作成者のメンバーシップ・
// そのメンバーシップへのロール割当を 1 トランザクションで作成し、管理するべき管理者の
// 居ないコミュニティ (や、属するコミュニティの無いロール・メンバーシップ) が決して
// 生じないようにします。
func (uc *CreateCommunityUsecase) createCommunity(ctx context.Context, input CreateCommunityInput) (*CreateCommunityOutput, error) {
	tx, err := uc.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("トランザクションの開始に失敗: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	communityRepo := uc.communityRepo.WithTx(tx)
	communityRoleRepo := uc.communityRoleRepo.WithTx(tx)
	communityMemberRepo := uc.communityMemberRepo.WithTx(tx)
	communityMemberRoleRepo := uc.communityMemberRoleRepo.WithTx(tx)

	community, err := communityRepo.Create(ctx, repository.CreateCommunityInput{
		Name:       input.Name,
		Identifier: input.Identifier,
	})
	if err != nil {
		return nil, fmt.Errorf("コミュニティの作成に失敗: %w", err)
	}

	role, err := communityRoleRepo.Create(ctx, repository.CreateCommunityRoleInput{
		CommunityID: community.ID,
		Name:        administratorRoleName,
	})
	if err != nil {
		return nil, fmt.Errorf("管理者ロールの作成に失敗: %w", err)
	}

	member, err := communityMemberRepo.Create(ctx, repository.CreateCommunityMemberInput{
		CommunityID: community.ID,
		UserID:      input.UserID,
	})
	if err != nil {
		return nil, fmt.Errorf("作成者のメンバーシップの作成に失敗: %w", err)
	}

	if _, err := communityMemberRoleRepo.Create(ctx, repository.CreateCommunityMemberRoleInput{
		CommunityID:       community.ID,
		CommunityMemberID: member.ID,
		CommunityRoleID:   role.ID,
	}); err != nil {
		return nil, fmt.Errorf("作成者への管理者ロールの割当に失敗: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("トランザクションのコミットに失敗: %w", err)
	}

	return &CreateCommunityOutput{Community: community}, nil
}
