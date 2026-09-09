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

	// CanAccessAdmin reports whether the visitor the navigation was read for may
	// open the administration screens, which is what decides whether the sidebar
	// draws the link into them. It is false for an anonymous visitor, who holds no
	// role.
	//
	// [Ja] CanAccessAdmin は、このナビゲーションを読んだ訪問者が管理画面を開いてよいか
	// どうかを表し、サイドバーがそこへの導線を描くかどうかはこれで決まります。匿名の
	// 訪問者はロールを 1 つも持たないため、これは偽になります。
	CanAccessAdmin bool
}

// GetCommunityNavigationInput is the input to Execute: who the sidebar is being
// drawn for.
//
// UserID is nil for an anonymous visitor. The community's pages are readable
// without an account, so most of what the sidebar holds does not depend on who is
// looking, and the id arrives only for the one part that does.
//
// [Ja] GetCommunityNavigationInput は Execute の入力で、サイドバーが誰のために描かれる
// かを表します。
//
// UserID は匿名の訪問者のとき nil です。コミュニティのページはアカウント無しで読める
// ため、サイドバーが持つものの大半は誰が見ているかに依らず、id は依る部分のためだけに
// 届きます。
type GetCommunityNavigationInput struct {
	UserID *model.UserID
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
	roleRepo      *repository.RoleRepository
}

// NewGetCommunityNavigationUsecase builds a GetCommunityNavigationUsecase over
// the community and board repositories, together with the role repository the
// visitor's permission is resolved through.
//
// [Ja] NewGetCommunityNavigationUsecase はコミュニティと掲示板の各リポジトリ、および
// 訪問者の権限を解決するために通すロールのリポジトリから
// GetCommunityNavigationUsecase を構築します。
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

// Execute reads the community and the boards it offers, whichever category each
// board sits in and whether it sits in one at all, and answers whether the
// visitor may open the administration screens.
//
// The sidebar costs two queries on every page of the shell, and the number of
// categories does not enter into it. A signed-in visitor costs a third for the
// roles they hold; an anonymous one does not, since holding no account is
// already the whole answer and the community's public pages are the ones an
// anonymous visitor reads most.
//
// [Ja] Execute はコミュニティと、それが提供する掲示板を読み (掲示板がどのカテゴリーに
// 属するか、そもそも属するかどうかは問いません)、訪問者が管理画面を開いてよいかどうかを
// 答えます。
//
// サイドバーはシェルを持つどのページでも 2 クエリで済み、カテゴリーの数はそこに関わり
// ません。サインイン済みの訪問者は、その人が持つロールのために 3 つ目を払います。匿名の
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
