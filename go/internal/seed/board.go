package seed

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
)

// boardActivity says how much traffic a board is generated with. A board is
// described by what it has to show rather than by a number of threads: the
// counts live in contentPlan, which a test replaces with smaller ones, while
// which board is the busy one stays the same so that a screen is found in the
// same place from one run to the next.
//
// [Ja] boardActivity は掲示板をどれだけの賑わいで生成するかを述べます。掲示板を
// スレッド数ではなく「何を見せる掲示板か」で記述するのは、件数が contentPlan にあり、
// テストがそれをより小さいものへ差し替えるためです。どの掲示板が賑わっているかのほうは
// 実行をまたいで変わらず、目当ての画面が毎回同じ場所で見つかります。
type boardActivity int

const (
	// boardEmpty leaves a board without a single thread. It is the state every
	// board starts in, and the only one an empty state can be looked at in.
	//
	// [Ja] boardEmpty は掲示板をスレッドが 1 つも無いままにします。すべての掲示板が
	// 最初に置かれている状態であり、空状態を眺められる唯一の状態です。
	boardEmpty boardActivity = iota

	// boardQuiet gives a board the handful of threads that make a board list
	// read as a board list.
	//
	// [Ja] boardQuiet は、スレッド一覧がスレッド一覧として読める程度の数のスレッドを
	// 掲示板に与えます。
	boardQuiet

	// boardBusy gives a board more threads than one page of a thread list holds,
	// so that the pagination (M4) has something to page through. It is also
	// where the threads written to be opened one by one are posted.
	//
	// [Ja] boardBusy は、スレッド一覧の 1 ページに収まらない数のスレッドを掲示板に
	// 与え、ページネーション (M4) に捲るものがある状態にします。1 つずつ開いて眺める
	// ために書かれたスレッドが立つのもこの掲示板です。
	boardBusy
)

// seedCategory is one category of the generated community, and seedBoard is one
// of the boards it offers.
//
// [Ja] seedCategory は生成するコミュニティのカテゴリー 1 つ、seedBoard はそのコミュニティが
// 提供する掲示板 1 つを表します。
type seedCategory struct {
	slug string
	name string
}

type seedBoard struct {
	// categorySlug names the category listing this board, and is empty for a
	// board the community places outside every category (ADR 0011).
	//
	// [Ja] categorySlug はこの掲示板を並べるカテゴリーを名指し、コミュニティがどの
	// カテゴリーにも属さない形で置く掲示板では空です (ADR 0011)。
	categorySlug string

	slug        string
	name        string
	description string
	activity    boardActivity
}

// matureCategories and matureBoards are the community the mature profile
// generates. They are written out rather than generated because a category and a
// board are what the people running a community decide, and because a developer
// reaches a board by its slug: a run that renamed them would leave the addresses
// in a browser's history pointing at nothing.
//
// The boards are one flat list rather than nested under the categories that
// list them, because that is the shape the sidebar draws (ADR 0011) and because
// a board outside every category has no category to be nested under. The order
// here is the order the sidebar shows, which is what the position of each row is
// set from. A board with no category sits between ones that have one, so that
// the two are seen mixed rather than in separate runs.
//
// [Ja] matureCategories と matureBoards は、mature プロファイルが生成するコミュニティ
// です。生成せずに書き下しているのは、カテゴリーと掲示板がコミュニティを運営する人の
// 決めるものであることと、開発者が掲示板へ slug で辿り着くことによります。実行のたびに
// 名前が変われば、ブラウザの履歴に残ったアドレスは何も指さなくなります。
//
// 掲示板をそれを並べるカテゴリーの下に入れ子にせず 1 つの平坦な一覧にしているのは、それが
// サイドバーの描く形であり (ADR 0011)、どのカテゴリーにも属さない掲示板には入れ子になる
// 先が無いためです。ここでの順序はサイドバーが見せる順序であり、各行の position はこれを
// もとに設定します。カテゴリーを持たない掲示板は持つ掲示板の間に置き、両者が別々に
// 固まらず混ざった状態で見えるようにしています。
var matureCategories = []seedCategory{
	{slug: "community", name: "コミュニティ"},
	{slug: "hobby", name: "趣味"},
}

var matureBoards = []seedBoard{
	{
		categorySlug: "community",
		slug:         "chat",
		name:         "雑談",
		description:  "話題を決めずに書き込む場所です。",
		activity:     boardBusy,
	},
	{
		categorySlug: "community",
		slug:         "announcements",
		name:         "お知らせ",
		description:  "運営からの連絡を掲示します。",
		activity:     boardEmpty,
	},
	{
		slug:        "questions",
		name:        "質問",
		description: "分からないことを尋ねます。",
		activity:    boardQuiet,
	},
	{
		categorySlug: "hobby",
		slug:         "games",
		name:         "ゲーム",
		description:  "遊んでいるゲームの話をします。",
		activity:     boardQuiet,
	},
	{
		categorySlug: "hobby",
		slug:         "music",
		name:         "音楽",
		description:  "聴いている音楽の話をします。",
		activity:     boardQuiet,
	},
}

// coldStartBoards is the community the cold-start profile generates: the one
// board an instance opens with, filed under no category at all. A community
// starts from the board that takes everything, and a category is drawn once
// there are boards to group; putting either up in advance leaves an empty
// container nobody but the person who made it has a reason to open (ADR 0010,
// ADR 0011).
//
// The board carries the slug the mature community's busy board carries, so that
// an address noted down while looking at one state still opens a board in the
// other.
//
// [Ja] coldStartBoards は cold-start プロファイルが生成するコミュニティです。
// インスタンスが開くときに持つ掲示板 1 つだけで、どのカテゴリーにも属しません。
// コミュニティはすべてを受け止める掲示板から始まり、カテゴリーはまとめる対象の掲示板が
// できてから描かれます。どちらも先回りして用意すれば、作った人以外に開く理由の無い空の
// 器が残ります (ADR 0010・ADR 0011)。
//
// この掲示板は成熟したコミュニティの賑わう掲示板と同じ slug を持ちます。片方の状態を
// 眺めながら書き留めたアドレスが、もう片方でも掲示板を開くようにするためです。
var coldStartBoards = []seedBoard{
	{
		slug:        "chat",
		name:        "雑談",
		description: "話題を決めずに書き込む場所です。",
		activity:    boardQuiet,
	},
}

// seededBoard is a board a run created, carrying the activity it was described
// with so that the generator that fills it knows how much to write.
//
// [Ja] seededBoard は実行が作成した掲示板です。それを埋める生成器がどれだけ書くのかを
// 知れるよう、記述に使った賑わいを一緒に運びます。
type seededBoard struct {
	board    *model.Board
	activity boardActivity
}

// generateBoards creates the categories and the boards the community offers.
//
// [Ja] generateBoards はカテゴリーと、コミュニティが提供する掲示板を作成します。
func (r *Runner) generateBoards(ctx context.Context, tx *sql.Tx, st *state) error {
	if err := validateSeedSlugs(r.profile.categories, r.profile.boards); err != nil {
		return err
	}

	bar := newProgress(r.out, "boards", len(r.profile.boards))
	defer bar.finish()

	categoryRepo := repository.NewCategoryRepository(r.db).WithTx(tx)
	boardRepo := repository.NewBoardRepository(r.db).WithTx(tx)

	categoryIDs := make(map[string]model.CategoryID, len(r.profile.categories))
	for i, category := range r.profile.categories {
		createdCategory, err := categoryRepo.Create(ctx, repository.CreateCategoryInput{
			Slug:     category.slug,
			Name:     category.name,
			Position: i,
		})
		if err != nil {
			return fmt.Errorf("failed to create the category %s: %w", category.slug, err)
		}

		categoryIDs[category.slug] = createdCategory.ID
	}

	for i, board := range r.profile.boards {
		var categoryID *model.CategoryID
		if board.categorySlug != "" {
			id := categoryIDs[board.categorySlug]
			categoryID = &id
		}

		createdBoard, err := boardRepo.Create(ctx, repository.CreateBoardInput{
			CategoryID:  categoryID,
			Slug:        board.slug,
			Name:        board.name,
			Description: board.description,
			Position:    i,
		})
		if err != nil {
			return fmt.Errorf("failed to create the board %s: %w", board.slug, err)
		}

		st.boards = append(st.boards, seededBoard{board: createdBoard, activity: board.activity})
		bar.advance()
	}

	return nil
}

// validateSeedSlugs checks the written-out slugs against the rule the
// application applies to every category and board, rather than against a copy of
// it here, so that a slug this file introduces is one /c/{slug} and /b/{slug}
// can address. templates.BoardPath places a slug into a path as written, so a
// slug carrying a path or query character would produce a link pointing
// somewhere other than the board.
//
// It also checks that a board naming a category names one this file writes.
// A board is left outside every category by writing no category at all, so a
// misspelled name that resolved to nothing would silently produce that state
// instead of failing.
//
// [Ja] validateSeedSlugs は書き下した slug を、ここに置いた写しではなくアプリケーションが
// すべてのカテゴリーと掲示板に課している規則で検査します。本ファイルが持ち込む slug が
// /c/{slug} と /b/{slug} で指せるものであるようにするためです。templates.BoardPath は
// slug をそのままパスへ置くため、パスやクエリの文字を含む slug は掲示板ではないどこかを
// 指すリンクを作ってしまいます。
//
// 併せて、掲示板が名指すカテゴリーが本ファイルの書き下すものであることも検査します。
// どのカテゴリーにも属さない掲示板はカテゴリーを何も書かないことで表すため、綴りを誤って
// 何も解決しない名前は、失敗せずに黙ってその状態を作ってしまいます。
func validateSeedSlugs(categories []seedCategory, boards []seedBoard) error {
	knownCategories := make(map[string]bool, len(categories))
	for _, category := range categories {
		if !model.IsValidSlug(category.slug) {
			return fmt.Errorf(
				"the category slug %q may hold only lowercase ASCII letters, digits, hyphens and underscores, and at most %d of them",
				category.slug, model.SlugMaxLength,
			)
		}
		knownCategories[category.slug] = true
	}

	for _, board := range boards {
		if !model.IsValidSlug(board.slug) {
			return fmt.Errorf(
				"the board slug %q may hold only lowercase ASCII letters, digits, hyphens and underscores, and at most %d of them",
				board.slug, model.SlugMaxLength,
			)
		}
		if board.categorySlug != "" && !knownCategories[board.categorySlug] {
			return fmt.Errorf("the board %q names the category %q, which no category is written with", board.slug, board.categorySlug)
		}
	}

	return nil
}
