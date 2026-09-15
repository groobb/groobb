package usecase

import (
	"context"
	"fmt"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
)

// GetThreadInputは読み取るスレッドを、/t/{id} が運ぶidで指定します。slugでは
// なくidで名指すのは、タイトルが編集されうるためで、タイトルから導いたアドレスでは
// 既に共有されたリンクが壊れます。
type GetThreadInput struct {
	ID model.ThreadID

	// UserIDはページが誰のために読まれるかで、匿名の訪問者のときはnilです。スレッドの
	// 持つものは誰にとっても同じであるため、idが届くのはそうでない唯一の部分、すなわち
	// ページが、管理者がスレッドに対して働きかける操作を差し出すかどうかのためです。
	UserID *model.UserID
}

// ThreadPostはスレッドの投稿1つと、投稿の行ではなくスレッドが知っていること、
// すなわち誰が書いたかと、後続のどの投稿がそれに答えたかを合わせて持ちます。
type ThreadPost struct {
	Post *model.Post

	// Authorは投稿を書いたアカウントで、解決できるものが無いときはnilです。
	// アカウントが退会したか、その行が既にパージされたかのいずれかです。どちらの場合も
	// 投稿は残るため、表示は投稿を落とすのではなく作者が居なくなったことを述べます。
	Author *model.User

	// ReplyNumbersはこの投稿を参照する投稿のレス番号を、書かれた順に持ちます。
	// idではなく番号であるのは、スレッドの中で投稿を指すのがレス番号であり、そこへ
	// 戻るリンクがそれを元に組み立てられるためです。
	ReplyNumbers []int
}

// GetThreadOutputはスレッドのページです。スレッド、それが立った掲示板と、その
// 掲示板を並べるカテゴリー、そしてその中のすべての投稿をレス番号順に持ちます。
//
// 掲示板とカテゴリーが伴うのは、/t/{id} がスレッドをその在り処を言わずに名指しするため、
// ページ側がそれを述べる必要があるからです。掲示板のページが自身のカテゴリーを運ぶのと
// 同じです。掲示板がどのカテゴリーにも属さないときはCategoryがnilになります。これは
// 欠落ではなく正常な状態です (ADR 0011)。投稿の一覧が丸ごと伴うのは、スレッドが丸ごと
// 配信されるためです (ADR 0009)。
type GetThreadOutput struct {
	Thread   *model.Thread
	Board    *model.Board
	Category *model.Category
	Posts    []ThreadPost

	// CanLockThreadは、このページを読んだ訪問者がこのスレッドのロックを扱ってよいか
	// どうか、すなわち掛けることと外すことの双方を返します。1つのスコープがこの対を許す
	// ため (ADR 0013)、1つの答えが両方を覆います。2つのうちどちらをページが差し出すかは、
	// ここにもう1つの答えを置くのではなく、スレッドが持つロックが決めます。
	CanLockThread bool

	// CanUnpublishThreadは、訪問者がこのスレッドをコミュニティの視界から外して
	// よいかどうかを返します。
	CanUnpublishThread bool

	// CanUnpublishPostは、訪問者が投稿1件を視界から外してよいかどうかを返します。
	// 下に並ぶ投稿がその操作を持つかどうかを決めるものがこれです。
	CanUnpublishPost bool
}

// GetThreadUsecaseはスレッドのページが描かれる元をすべて読みます。読み取り
// UseCaseであり、リポジトリの取得系メソッドしか呼ばないため、validatorもトランザク
// ションも必要としません。
//
// 掲示板のページと違い、こちらは件数の決まった解決と件数に上限の無い一覧に分けていま
// せん。/t/{id} には、描画とリダイレクトのどちらを選ぶかを決めるルックアップがありま
// せん。idが数の正規の綴りであるかどうかだけであり、それはハンドラーが何も読まずに
// 決めます。Executeは投稿一覧を読む前にスレッドの不在と非公開を判定し、どちらの場合も
// 投稿を1件も読まないうちに応答します。
type GetThreadUsecase struct {
	threadRepo        *repository.ThreadRepository
	boardRepo         *repository.BoardRepository
	categoryRepo      *repository.CategoryRepository
	postRepo          *repository.PostRepository
	postReferenceRepo *repository.PostReferenceRepository
	userRepo          *repository.UserRepository
	roleRepo          *repository.RoleRepository
}

// NewGetThreadUsecaseは、スレッドのページが読み取る各リポジトリと、サインイン済みの
// 訪問者の権限を解決するために通すロールのリポジトリからGetThreadUsecaseを構築します。
func NewGetThreadUsecase(
	threadRepo *repository.ThreadRepository,
	boardRepo *repository.BoardRepository,
	categoryRepo *repository.CategoryRepository,
	postRepo *repository.PostRepository,
	postReferenceRepo *repository.PostReferenceRepository,
	userRepo *repository.UserRepository,
	roleRepo *repository.RoleRepository,
) *GetThreadUsecase {
	return &GetThreadUsecase{
		threadRepo:        threadRepo,
		boardRepo:         boardRepo,
		categoryRepo:      categoryRepo,
		postRepo:          postRepo,
		postReferenceRepo: postReferenceRepo,
		userRepo:          userRepo,
		roleRepo:          roleRepo,
	}
}

// Executeはスレッドを解決し、その在り処を読み、投稿を作者と、それを指して戻る
// 返信とともに組み立てます。
//
// どのスレッドも指さないidはAppErrCodeResourceNotFoundを持つAppErrorとして報告し、
// ハンドラーが共通のnot-foundページで404を返せるようにします。これは推測された、
// あるいは削除されたスレッドの残したURLの既知の結果であって失敗ではないため、ここでは
// エラーとしてログに残しません。
//
// 管理者が非公開にしたスレッドを指すidはAppErrCodeResourceUnpublishedとして報告し、Executeは
// 投稿を1件も読まないうちに返ります。スレッドに付いた印がその下の会話を隠すため、このページが
// 組み立てるものは残っていないためです。2つを分けるのは、ハンドラーがスレッドの取り下げを
// 述べられるようにするためです。そうしなければ、共有されたリンクを手にした訪問者に、それを
// 打ち間違えたのかどうかを考えさせることになります。
//
// 一方、掲示板を読み戻せない場合、および掲示板がカテゴリーを名指しているのにそれを
// 読み戻せない場合は失敗です。スレッドは必ず掲示板に属し、掲示板の削除はそのスレッドを
// 道連れにします。カテゴリーの削除は名指しを、消えた行を指したままにするのではなく空に
// します。どちらかをページの不在として報告すれば、まだ存在するスレッドを落とすよう
// クローラーに伝えてしまいます。
//
// 作者と参照はいずれも投稿ごとではなく全投稿分をまとめて読むため、スレッドが投稿1件を
// 持つ場合でも上限の1000件を持つ場合でも、ページのクエリ数は一定です。
//
// サインイン済みの訪問者はクエリを1つ多く払います。スレッドをモデレートしてよいかどうかを
// 読む元となるロールのためです。匿名の訪問者は払いません。アカウントを持たないことが
// すでに答えのすべてであり、スレッドのページは匿名の訪問者が最も多く読むページであるため
// です。ロールはスレッドを解決した後、最後に読みます。不在のスレッドと非公開のスレッドには、
// その支払いをせずに応答するためです。
func (uc *GetThreadUsecase) Execute(ctx context.Context, input GetThreadInput) (*GetThreadOutput, error) {
	thread, err := uc.threadRepo.FindByID(ctx, input.ID)
	if err != nil {
		return nil, fmt.Errorf("スレッドの取得に失敗: %w", err)
	}
	if thread == nil {
		return nil, &model.AppError{
			Code:     model.AppErrCodeResourceNotFound,
			UserMsg:  i18n.T(ctx, "error_not_found_message"),
			Internal: fmt.Errorf("スレッドが見つからない: id=%s", input.ID),
			Metadata: map[string]string{"thread_id": input.ID.String()},
		}
	}
	if thread.UnpublishedAt != nil {
		return nil, &model.AppError{
			Code:     model.AppErrCodeResourceUnpublished,
			UserMsg:  i18n.T(ctx, "error_unpublished_message"),
			Internal: fmt.Errorf("非公開のスレッドの読み取り: id=%s", input.ID),
			Metadata: map[string]string{"thread_id": input.ID.String()},
		}
	}

	board, err := uc.boardRepo.FindByID(ctx, thread.BoardID)
	if err != nil {
		return nil, fmt.Errorf("スレッドの掲示板の取得に失敗: %w", err)
	}
	if board == nil {
		return nil, fmt.Errorf("スレッドの掲示板が見つからない: thread_id=%s board_id=%s", thread.ID, thread.BoardID)
	}

	category, err := uc.findBoardCategory(ctx, board)
	if err != nil {
		return nil, err
	}

	posts, err := uc.postRepo.ListByThreadID(ctx, thread.ID)
	if err != nil {
		return nil, fmt.Errorf("スレッドの投稿一覧の取得に失敗: %w", err)
	}

	references, err := uc.postReferenceRepo.ListByReferencedPostIDs(ctx, postIDs(posts))
	if err != nil {
		return nil, fmt.Errorf("投稿の逆参照の取得に失敗: %w", err)
	}

	authors, err := uc.userRepo.ListByIDs(ctx, authorIDs(posts))
	if err != nil {
		return nil, fmt.Errorf("投稿の作者の取得に失敗: %w", err)
	}

	output := &GetThreadOutput{
		Thread:   thread,
		Board:    board,
		Category: category,
		Posts:    assembleThreadPosts(posts, references, authors),
	}

	if input.UserID != nil {
		communityPolicy, err := resolveCommunityPolicy(ctx, uc.roleRepo, UserActor(*input.UserID))
		if err != nil {
			return nil, err
		}
		output.CanLockThread = communityPolicy.CanLockThread()
		output.CanUnpublishThread = communityPolicy.CanUnpublishThread()
		output.CanUnpublishPost = communityPolicy.CanUnpublishPost()
	}

	return output, nil
}

// findBoardCategoryは指定された掲示板が名指すカテゴリーを読み、どのカテゴリーも
// 名指していない掲示板にはnilを返します。
func (uc *GetThreadUsecase) findBoardCategory(ctx context.Context, board *model.Board) (*model.Category, error) {
	if board.CategoryID == nil {
		return nil, nil
	}

	category, err := uc.categoryRepo.FindByID(ctx, *board.CategoryID)
	if err != nil {
		return nil, fmt.Errorf("掲示板のカテゴリーの取得に失敗: %w", err)
	}
	if category == nil {
		return nil, fmt.Errorf("掲示板のカテゴリーが見つからない: board_id=%s category_id=%s", board.ID, *board.CategoryID)
	}
	return category, nil
}

// postIDsは、参照を引く手がかりとなる投稿のidを集めます。
func postIDs(posts []*model.Post) []model.PostID {
	ids := make([]model.PostID, len(posts))
	for i, post := range posts {
		ids[i] = post.ID
	}
	return ids
}

// authorIDsは解決するアカウントを、そのアカウントが何件書いていても1度ずつ
// 集めます。作者の行が既にパージされた投稿はidを持たず、何も足しません。
func authorIDs(posts []*model.Post) []model.UserID {
	seen := make(map[model.UserID]bool, len(posts))
	var ids []model.UserID
	for _, post := range posts {
		if post.UserID == nil || seen[*post.UserID] {
			continue
		}
		seen[*post.UserID] = true
		ids = append(ids, *post.UserID)
	}
	return ids
}

// assembleThreadPostsは3つの読み取りを、ページが描画する投稿へと繋ぎます。
// 各投稿と、それを書いたアカウントと、それを参照した投稿のレス番号です。
//
// 参照はidの対として届き、レス番号として出ていきます。スレッドの中で投稿を指すのが
// その番号だからです。どの対も両端がこの同じスレッドの投稿であるため、番号はすべて手元に
// あります。post_referencesは >>Nから書かれ、それが名指すのは常に、参照した投稿が
// 属するスレッドの投稿だからです。
func assembleThreadPosts(posts []*model.Post, references []*model.PostReference, authors []*model.User) []ThreadPost {
	authorByID := make(map[model.UserID]*model.User, len(authors))
	for _, author := range authors {
		authorByID[author.ID] = author
	}

	numberByPostID := make(map[model.PostID]int, len(posts))
	for _, post := range posts {
		numberByPostID[post.ID] = post.Number
	}

	replyNumbers := make(map[model.PostID][]int, len(references))
	for _, reference := range references {
		number, ok := numberByPostID[reference.PostID]
		if !ok {
			continue
		}
		replyNumbers[reference.ReferencedPostID] = append(replyNumbers[reference.ReferencedPostID], number)
	}

	threadPosts := make([]ThreadPost, len(posts))
	for i, post := range posts {
		threadPost := ThreadPost{Post: post, ReplyNumbers: replyNumbers[post.ID]}
		if post.UserID != nil {
			threadPost.Author = authorByID[*post.UserID]
		}
		threadPosts[i] = threadPost
	}
	return threadPosts
}
