package usecase

import (
	"context"
	"fmt"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
)

// GetCommunityNavigationOutput is what every page of the community shell needs
// to draw its sidebar: the name of this community and the boards it offers.
//
// The boards come back as one flat list rather than grouped under the categories
// that list them, because that is how the sidebar draws them: a community whose
// boards number a handful gains nothing from headings with one board each, and
// the hierarchy arrives later as something its operators switch on (ADR 0011).
//
// Community is nil when the instance has not been set up yet, so the sidebar
// draws the board navigation without a name rather than an empty heading.
//
// [Ja] GetCommunityNavigationOutput は、コミュニティのシェルを持つどのページも
// サイドバーを描くために必要とするもの、すなわちこのコミュニティの名前と、それが
// 提供する掲示板です。
//
// 掲示板をそれを並べるカテゴリーの下に束ねず 1 つの平坦な一覧として返すのは、
// サイドバーがそのように描くためです。掲示板が数個のコミュニティは、掲示板 1 つずつの
// 見出しからは何も得られず、階層は後から運営が切り替えるものとして入ります (ADR 0011)。
//
// Community はインスタンスがまだ立ち上げられていないとき nil になります。この場合
// サイドバーは空の見出しではなく、名前を持たない板のナビゲーションを描きます。
type GetCommunityNavigationOutput struct {
	Community *model.Community
	Boards    []*model.Board
}

// GetCommunityNavigationUsecase gathers the sidebar's contents. It is a read
// UseCase: it only calls the lookup methods of its repositories, so it needs
// neither a validator nor a transaction.
//
// [Ja] GetCommunityNavigationUsecase はサイドバーの内容を集めます。読み取り UseCase で
// あり、リポジトリの取得系メソッドしか呼ばないため、validator もトランザクションも
// 必要としません。
type GetCommunityNavigationUsecase struct {
	communityRepo *repository.CommunityRepository
	boardRepo     *repository.BoardRepository
}

// NewGetCommunityNavigationUsecase builds a GetCommunityNavigationUsecase over
// the community and board repositories.
//
// [Ja] NewGetCommunityNavigationUsecase はコミュニティと掲示板の各リポジトリから
// GetCommunityNavigationUsecase を構築します。
func NewGetCommunityNavigationUsecase(
	communityRepo *repository.CommunityRepository,
	boardRepo *repository.BoardRepository,
) *GetCommunityNavigationUsecase {
	return &GetCommunityNavigationUsecase{
		communityRepo: communityRepo,
		boardRepo:     boardRepo,
	}
}

// Execute reads the community and the boards it offers, whichever category each
// board sits in and whether it sits in one at all.
//
// The sidebar costs two queries on every page of the shell, and the number of
// categories does not enter into it.
//
// [Ja] Execute はコミュニティと、それが提供する掲示板を読みます。掲示板がどのカテゴリーに
// 属するか、そもそも属するかどうかは問いません。
//
// サイドバーはシェルを持つどのページでも 2 クエリで済み、カテゴリーの数はそこに関わり
// ません。
func (uc *GetCommunityNavigationUsecase) Execute(ctx context.Context) (*GetCommunityNavigationOutput, error) {
	community, err := uc.communityRepo.Find(ctx)
	if err != nil {
		return nil, fmt.Errorf("コミュニティの取得に失敗: %w", err)
	}

	boards, err := uc.boardRepo.ListAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("掲示板一覧の取得に失敗: %w", err)
	}

	return &GetCommunityNavigationOutput{Community: community, Boards: boards}, nil
}
