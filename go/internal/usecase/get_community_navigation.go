package usecase

import (
	"context"
	"fmt"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
)

// GetCommunityNavigationOutputは、コミュニティのシェルを持つどのページも
// サイドバーを描くために必要とするもの、すなわちこのコミュニティの名前と、それが
// 提供する掲示板です。
//
// 掲示板をそれを並べるカテゴリーの下に束ねず1つの平坦な一覧として返すのは、
// サイドバーがそのように描くためです。掲示板が数個のコミュニティは、掲示板1つずつの
// 見出しからは何も得られず、階層は後から運営が切り替えるものとして入ります (ADR 0011)。
//
// Communityはインスタンスがまだ立ち上げられていないときnilになります。この場合
// サイドバーは空の見出しではなく、名前を持たない板のナビゲーションを描きます。
type GetCommunityNavigationOutput struct {
	Community *model.Community
	Boards    []*model.Board

	// CanAccessAdminは、このナビゲーションを読んだ訪問者が管理画面を開いてよいか
	// どうかを表し、サイドバーがそこへの導線を描くかどうかはこれで決まります。匿名の
	// 訪問者はロールを1つも持たないため、これは偽になります。
	CanAccessAdmin bool
}

// GetCommunityNavigationInputはExecuteの入力で、サイドバーが誰のために描かれる
// かを表します。
//
// UserIDは匿名の訪問者のときnilです。コミュニティのページはアカウント無しで読める
// ため、サイドバーが持つものの大半は誰が見ているかに依らず、idは依る部分のためだけに
// 届きます。
type GetCommunityNavigationInput struct {
	UserID *model.UserID
}

// GetCommunityNavigationUsecaseはサイドバーの内容を集めます。読み取りUseCaseで
// あり、リポジトリの取得系メソッドしか呼ばないため、validatorもトランザクションも
// 必要としません。
type GetCommunityNavigationUsecase struct {
	communityRepo *repository.CommunityRepository
	boardRepo     *repository.BoardRepository
	roleRepo      *repository.RoleRepository
}

// NewGetCommunityNavigationUsecaseはコミュニティと掲示板の各リポジトリ、および
// 訪問者の権限を解決するために通すロールのリポジトリから
// GetCommunityNavigationUsecaseを構築します。
func NewGetCommunityNavigationUsecase(
	communityRepo *repository.CommunityRepository,
	boardRepo *repository.BoardRepository,
	roleRepo *repository.RoleRepository,
) *GetCommunityNavigationUsecase {
	return &GetCommunityNavigationUsecase{
		communityRepo: communityRepo,
		boardRepo:     boardRepo,
		roleRepo:      roleRepo,
	}
}

// Executeはコミュニティと、それが提供する掲示板を読み (掲示板がどのカテゴリーに
// 属するか、そもそも属するかどうかは問いません)、訪問者が管理画面を開いてよいかどうかを
// 答えます。
//
// サイドバーはシェルを持つどのページでも2クエリで済み、カテゴリーの数はそこに関わり
// ません。サインイン済みの訪問者は、その人が持つロールのために3つ目を払います。匿名の
// 訪問者は払いません。アカウントを持たないことがすでに答えのすべてであり、コミュニティの
// 公開ページは匿名の訪問者が最も多く読むページであるためです。
func (uc *GetCommunityNavigationUsecase) Execute(
	ctx context.Context,
	input GetCommunityNavigationInput,
) (*GetCommunityNavigationOutput, error) {
	community, err := uc.communityRepo.Find(ctx)
	if err != nil {
		return nil, fmt.Errorf("コミュニティの取得に失敗: %w", err)
	}

	boards, err := uc.boardRepo.ListAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("掲示板一覧の取得に失敗: %w", err)
	}

	output := &GetCommunityNavigationOutput{Community: community, Boards: boards}

	if input.UserID != nil {
		communityPolicy, err := resolveCommunityPolicy(ctx, uc.roleRepo, UserActor(*input.UserID))
		if err != nil {
			return nil, err
		}
		output.CanAccessAdmin = communityPolicy.CanAccessAdmin()
	}

	return output, nil
}
