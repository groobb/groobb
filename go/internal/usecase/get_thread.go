package usecase

import (
	"context"
	"fmt"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
)

// GetThreadInput addresses the thread to read by the id /t/{id} carries. A
// thread is named by id rather than by a slug because its title can be edited,
// and an address derived from the title would break the links already shared.
//
// [Ja] GetThreadInput は読み取るスレッドを、/t/{id} が運ぶ id で指定します。slug では
// なく id で名指すのは、タイトルが編集されうるためで、タイトルから導いたアドレスでは
// 既に共有されたリンクが壊れます。
type GetThreadInput struct {
	ID model.ThreadID
}

// ThreadPost is one post of a thread together with what the thread, rather than
// the post row, knows about it: who wrote it, and which later posts replied to
// it.
//
// [Ja] ThreadPost はスレッドの投稿 1 つと、投稿の行ではなくスレッドが知っていること、
// すなわち誰が書いたかと、後続のどの投稿がそれに答えたかを合わせて持ちます。
type ThreadPost struct {
	Post *model.Post

	// Author is the account that wrote the post, and nil when there is none to
	// resolve: the account has withdrawn, or its row has since been purged. The
	// post stays either way, so the display says the author is gone rather than
	// leaving the post out.
	//
	// [Ja] Author は投稿を書いたアカウントで、解決できるものが無いときは nil です。
	// アカウントが退会したか、その行が既にパージされたかのいずれかです。どちらの場合も
	// 投稿は残るため、表示は投稿を落とすのではなく作者が居なくなったことを述べます。
	Author *model.User

	// ReplyNumbers are the reply numbers of the posts that reference this one,
	// in the order they were written. They are numbers rather than ids because a
	// reply number is what addresses a post inside its thread, which is what the
	// links pointing back at it are built from.
	//
	// [Ja] ReplyNumbers はこの投稿を参照する投稿のレス番号を、書かれた順に持ちます。
	// id ではなく番号であるのは、スレッドの中で投稿を指すのがレス番号であり、そこへ
	// 戻るリンクがそれを元に組み立てられるためです。
	ReplyNumbers []int
}

// GetThreadOutput is a thread's page: the thread, the board it was posted in
// and the category that lists that board, and every post in it in reply-number
// order.
//
// The board and the category come along because /t/{id} names a thread without
// naming where it sits, so the page has to say it, the same way a board's page
// carries its category. Category is nil when the board sits in none, which is a
// normal state rather than a gap (ADR 0011). The whole post list comes along
// because a thread is served whole (ADR 0009).
//
// [Ja] GetThreadOutput はスレッドのページです。スレッド、それが立った掲示板と、その
// 掲示板を並べるカテゴリー、そしてその中のすべての投稿をレス番号順に持ちます。
//
// 掲示板とカテゴリーが伴うのは、/t/{id} がスレッドをその在り処を言わずに名指しするため、
// ページ側がそれを述べる必要があるからです。掲示板のページが自身のカテゴリーを運ぶのと
// 同じです。掲示板がどのカテゴリーにも属さないときは Category が nil になります。これは
// 欠落ではなく正常な状態です (ADR 0011)。投稿の一覧が丸ごと伴うのは、スレッドが丸ごと
// 配信されるためです (ADR 0009)。
type GetThreadOutput struct {
	Thread   *model.Thread
	Board    *model.Board
	Category *model.Category
	Posts    []ThreadPost
}

// GetThreadUsecase reads everything a thread's page is drawn from. It is a read
// UseCase: it only calls the lookup methods of its repositories, so it needs
// neither a validator nor a transaction.
//
// Unlike a board's page, this one is not split into a bounded resolution and an
// unbounded listing. /t/{id} has no lookup that decides between rendering and
// redirecting — the id is either the canonical spelling of a number or it is
// not, which the handler settles without reading anything — so the only early
// answer is the missing thread, and Execute returns that before it reads a
// single post.
//
// [Ja] GetThreadUsecase はスレッドのページが描かれる元をすべて読みます。読み取り
// UseCase であり、リポジトリの取得系メソッドしか呼ばないため、validator もトランザク
// ションも必要としません。
//
// 掲示板のページと違い、こちらは件数の決まった解決と件数に上限の無い一覧に分けていま
// せん。/t/{id} には、描画とリダイレクトのどちらを選ぶかを決めるルックアップがありま
// せん。id が数の正規の綴りであるかどうかだけであり、それはハンドラーが何も読まずに
// 決めます。したがって早く返る応答はスレッドの不在だけであり、Execute は投稿を 1 件も
// 読まないうちにそれを返します。
type GetThreadUsecase struct {
	threadRepo        *repository.ThreadRepository
	boardRepo         *repository.BoardRepository
	categoryRepo      *repository.CategoryRepository
	postRepo          *repository.PostRepository
	postReferenceRepo *repository.PostReferenceRepository
	userRepo          *repository.UserRepository
}

// NewGetThreadUsecase builds a GetThreadUsecase over the repositories a thread's
// page is read from.
//
// [Ja] NewGetThreadUsecase は、スレッドのページが読み取る各リポジトリから
// GetThreadUsecase を構築します。
func NewGetThreadUsecase(
	threadRepo *repository.ThreadRepository,
	boardRepo *repository.BoardRepository,
	categoryRepo *repository.CategoryRepository,
	postRepo *repository.PostRepository,
	postReferenceRepo *repository.PostReferenceRepository,
	userRepo *repository.UserRepository,
) *GetThreadUsecase {
	return &GetThreadUsecase{
		threadRepo:        threadRepo,
		boardRepo:         boardRepo,
		categoryRepo:      categoryRepo,
		postRepo:          postRepo,
		postReferenceRepo: postReferenceRepo,
		userRepo:          userRepo,
	}
}

// Execute resolves the thread, reads where it sits, and assembles its posts with
// their authors and the replies pointing back at them.
//
// An id naming no thread is reported as an AppError carrying
// AppErrCodeResourceNotFound, which is what lets the handler answer 404 with the
// shared not-found page. It is a known outcome of a URL that was guessed or left
// behind by a deleted thread rather than a failure, so it is not logged as an
// error here.
//
// A board that cannot be read back, or a category a board names but that cannot
// be read back, is a failure instead: a thread always belongs to a board and
// deleting a board takes its threads with it, while deleting a category clears
// the naming rather than leaving it pointing at a row that is gone. Reporting
// either as a missing page would tell a crawler to drop a thread that is still
// there.
//
// The authors and the references are each read for every post at once rather
// than per post, so the page costs a fixed number of queries whether the thread
// holds one post or the thousand it is capped at.
//
// [Ja] Execute はスレッドを解決し、その在り処を読み、投稿を作者と、それを指して戻る
// 返信とともに組み立てます。
//
// どのスレッドも指さない id は AppErrCodeResourceNotFound を持つ AppError として報告し、
// ハンドラーが共通の not-found ページで 404 を返せるようにします。これは推測された、
// あるいは削除されたスレッドの残した URL の既知の結果であって失敗ではないため、ここでは
// エラーとしてログに残しません。
//
// 一方、掲示板を読み戻せない場合、および掲示板がカテゴリーを名指しているのにそれを
// 読み戻せない場合は失敗です。スレッドは必ず掲示板に属し、掲示板の削除はそのスレッドを
// 道連れにします。カテゴリーの削除は名指しを、消えた行を指したままにするのではなく空に
// します。どちらかをページの不在として報告すれば、まだ存在するスレッドを落とすよう
// クローラーに伝えてしまいます。
//
// 作者と参照はいずれも投稿ごとではなく全投稿分をまとめて読むため、スレッドが投稿 1 件を
// 持つ場合でも上限の 1000 件を持つ場合でも、ページのクエリ数は一定です。
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

	return &GetThreadOutput{
		Thread:   thread,
		Board:    board,
		Category: category,
		Posts:    assembleThreadPosts(posts, references, authors),
	}, nil
}

// findBoardCategory reads the category the given board names, and returns nil
// for a board naming none.
//
// [Ja] findBoardCategory は指定された掲示板が名指すカテゴリーを読み、どのカテゴリーも
// 名指していない掲示板には nil を返します。
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

// postIDs collects the ids of the posts the references are looked up by.
//
// [Ja] postIDs は、参照を引く手がかりとなる投稿の id を集めます。
func postIDs(posts []*model.Post) []model.PostID {
	ids := make([]model.PostID, len(posts))
	for i, post := range posts {
		ids[i] = post.ID
	}
	return ids
}

// authorIDs collects the accounts to resolve, each once however many posts they
// wrote. A post whose author row has already been purged carries no id and
// contributes none.
//
// [Ja] authorIDs は解決するアカウントを、そのアカウントが何件書いていても 1 度ずつ
// 集めます。作者の行が既にパージされた投稿は id を持たず、何も足しません。
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

// assembleThreadPosts joins the three reads into the posts the page renders:
// each post with the account that wrote it and the reply numbers of the posts
// that referenced it.
//
// The references arrive as pairs of ids and leave as reply numbers, because a
// post is addressed inside its thread by its number. Both ends of every pair are
// posts of this same thread, so the numbers are all in hand: post_references is
// written from a >>N, which only ever names a post of the thread the referring
// post is in.
//
// [Ja] assembleThreadPosts は 3 つの読み取りを、ページが描画する投稿へと繋ぎます。
// 各投稿と、それを書いたアカウントと、それを参照した投稿のレス番号です。
//
// 参照は id の対として届き、レス番号として出ていきます。スレッドの中で投稿を指すのが
// その番号だからです。どの対も両端がこの同じスレッドの投稿であるため、番号はすべて手元に
// あります。post_references は >>N から書かれ、それが名指すのは常に、参照した投稿が
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
