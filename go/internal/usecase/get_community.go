package usecase

import (
	"context"
	"fmt"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
)

// GetCommunityUsecase reads the community a URL identifier addresses, for the
// community page to render.
//
// [Ja] GetCommunityUsecase は URL 識別子が指すコミュニティを読み取り、コミュニティ画面が
// 描画できるようにします。
type GetCommunityUsecase struct {
	communityRepo *repository.CommunityRepository
}

// NewGetCommunityUsecase builds a GetCommunityUsecase from the repository it
// reads through.
//
// [Ja] NewGetCommunityUsecase は読み取りに使うリポジトリから GetCommunityUsecase を
// 構築します。
func NewGetCommunityUsecase(communityRepo *repository.CommunityRepository) *GetCommunityUsecase {
	return &GetCommunityUsecase{communityRepo: communityRepo}
}

// GetCommunityInput is the input to Execute. Identifier is the URL identifier
// taken from the request path, matched without regard to letter case.
//
// [Ja] GetCommunityInput は Execute の入力です。Identifier はリクエストパスから取り出す
// URL 識別子で、大文字小文字を区別せずに照合します。
type GetCommunityInput struct {
	Identifier string
}

// GetCommunityOutput carries the community the identifier addresses.
//
// [Ja] GetCommunityOutput は識別子が指すコミュニティを運びます。
type GetCommunityOutput struct {
	Community *model.Community
}

// Execute returns the community with the given identifier. A community nobody
// has claimed is a known business-level failure rather than a system error, so
// it comes back as an AppError the handler answers with 404: an identifier
// reaches us from a shared link, which may name a community that never existed.
//
// [Ja] Execute は指定された識別子のコミュニティを返します。誰も取得していない識別子は
// システムエラーではなく業務レベルの既知の失敗のため、ハンドラーが 404 で応答する
// AppError として返します。識別子は共有リンク経由で渡ってくるため、存在しなかった
// コミュニティを指していることがあるからです。
func (uc *GetCommunityUsecase) Execute(ctx context.Context, input GetCommunityInput) (*GetCommunityOutput, error) {
	community, err := uc.communityRepo.FindByIdentifier(ctx, input.Identifier)
	if err != nil {
		return nil, fmt.Errorf("コミュニティの取得に失敗: %w", err)
	}
	if community == nil {
		return nil, &model.AppError{
			Code:    model.AppErrCodeResourceNotFound,
			UserMsg: i18n.T(ctx, "error_not_found_message"),
		}
	}

	return &GetCommunityOutput{Community: community}, nil
}
