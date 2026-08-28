package usecase

import (
	"context"
	"fmt"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
)

// GetCommunityOutput is the community this instance hosts.
//
// Community is nil when the instance has not been set up yet. That is a normal
// answer rather than a failure: the row is created when the instance is set up,
// so a freshly migrated database has none and the caller renders what it can
// without the name.
//
// [Ja] GetCommunityOutput はこのインスタンスが運営するコミュニティです。
//
// Community はインスタンスがまだ立ち上げられていないとき nil になります。これは失敗では
// なく正常な答えです。行はインスタンスの立ち上げが作るため、マイグレーション直後の
// データベースには存在せず、呼び出し側は名前が無いなりに描画できるものを描画します。
type GetCommunityOutput struct {
	Community *model.Community
}

// GetCommunityUsecase reads the community this instance hosts. It is a read
// UseCase: it only calls the lookup methods of its repository, so it needs
// neither a validator nor a transaction.
//
// It reads the community alone where GetCommunityNavigationUsecase reads it
// together with the boards the sidebar lists, because its caller is the shared
// <head>, which every route renders — including the ones no sidebar is drawn on.
//
// [Ja] GetCommunityUsecase はこのインスタンスが運営するコミュニティを読みます。読み取り
// UseCase であり、リポジトリの取得系メソッドしか呼ばないため、validator もトランザク
// ションも必要としません。
//
// GetCommunityNavigationUsecase がサイドバーの並べる掲示板とともに読むのに対し、こちらが
// コミュニティだけを読むのは、呼び出し元が共通の <head> であり、どのルートもそれを描画する
// ためです。サイドバーを描かないページも含みます。
type GetCommunityUsecase struct {
	communityRepo *repository.CommunityRepository
}

// NewGetCommunityUsecase builds a GetCommunityUsecase over the community
// repository.
//
// [Ja] NewGetCommunityUsecase はコミュニティのリポジトリから GetCommunityUsecase を
// 構築します。
func NewGetCommunityUsecase(communityRepo *repository.CommunityRepository) *GetCommunityUsecase {
	return &GetCommunityUsecase{communityRepo: communityRepo}
}

// Execute reads the community this instance hosts. It takes no input because the
// instance serves exactly one community (ADR 0006), and it costs one query.
//
// [Ja] Execute はこのインスタンスが運営するコミュニティを読みます。インスタンスが運営する
// コミュニティはちょうど 1 つ (ADR 0006) のため入力を取らず、1 クエリで済みます。
func (uc *GetCommunityUsecase) Execute(ctx context.Context) (*GetCommunityOutput, error) {
	community, err := uc.communityRepo.Find(ctx)
	if err != nil {
		return nil, fmt.Errorf("コミュニティの取得に失敗: %w", err)
	}

	return &GetCommunityOutput{Community: community}, nil
}
