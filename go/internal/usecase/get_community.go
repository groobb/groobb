package usecase

import (
	"context"
	"fmt"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
)

// GetCommunityOutputはこのインスタンスが運営するコミュニティです。
//
// Communityはインスタンスがまだ立ち上げられていないときnilになります。これは失敗では
// なく正常な答えです。行はインスタンスの立ち上げが作るため、マイグレーション直後の
// データベースには存在せず、呼び出し側は名前が無いなりに描画できるものを描画します。
type GetCommunityOutput struct {
	Community *model.Community
}

// GetCommunityUsecaseはこのインスタンスが運営するコミュニティを読みます。読み取り
// UseCaseであり、リポジトリの取得系メソッドしか呼ばないため、validatorもトランザク
// ションも必要としません。
//
// GetCommunityNavigationUsecaseがサイドバーの並べる掲示板とともに読むのに対し、こちらが
// コミュニティだけを読むのは、呼び出し元が共通の <head> であり、どのルートもそれを描画する
// ためです。サイドバーを描かないページも含みます。
type GetCommunityUsecase struct {
	communityRepo *repository.CommunityRepository
}

// NewGetCommunityUsecaseはコミュニティのリポジトリからGetCommunityUsecaseを
// 構築します。
func NewGetCommunityUsecase(communityRepo *repository.CommunityRepository) *GetCommunityUsecase {
	return &GetCommunityUsecase{communityRepo: communityRepo}
}

// Executeはこのインスタンスが運営するコミュニティを読みます。インスタンスが運営する
// コミュニティはちょうど1つ (ADR 0006) のため入力を取らず、1クエリで済みます。
func (uc *GetCommunityUsecase) Execute(ctx context.Context) (*GetCommunityOutput, error) {
	community, err := uc.communityRepo.Find(ctx)
	if err != nil {
		return nil, fmt.Errorf("コミュニティの取得に失敗: %w", err)
	}

	return &GetCommunityOutput{Community: community}, nil
}
