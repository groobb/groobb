package seed

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
)

// boardActivityは掲示板をどれだけの賑わいで生成するかを述べます。掲示板を
// スレッド数ではなく「何を見せる掲示板か」で記述するのは、件数がcontentPlanにあり、
// テストがそれをより小さいものへ差し替えるためです。どの掲示板が賑わっているかのほうは
// 実行をまたいで変わらず、目当ての画面が毎回同じ場所で見つかります。
type boardActivity int

const (
	// boardEmptyは掲示板をスレッドが1つも無いままにします。すべての掲示板が
	// 最初に置かれている状態であり、空状態を眺められる唯一の状態です。
	boardEmpty boardActivity = iota

	// boardQuietは、スレッド一覧がスレッド一覧として読める程度の数のスレッドを
	// 掲示板に与えます。
	boardQuiet

	// boardBusyは、スレッド一覧の1ページに収まらない数のスレッドを掲示板に
	// 与え、ページネーション (M4) に捲るものがある状態にします。1つずつ開いて眺める
	// ために書かれたスレッドが立つのもこの掲示板です。
	boardBusy
)

// seedCategoryは生成するコミュニティのカテゴリー1つ、seedBoardはそのコミュニティが
// 提供する掲示板1つを表します。
type seedCategory struct {
	slug string
	name string
}

type seedBoard struct {
	// categorySlugはこの掲示板を並べるカテゴリーを名指し、コミュニティがどの
	// カテゴリーにも属さない形で置く掲示板では空です (ADR 0011)。
	categorySlug string

	slug        string
	name        string
	description string
	activity    boardActivity
}

// matureCategoriesとmatureBoardsは、matureプロファイルが生成するコミュニティ
// です。生成せずに書き下しているのは、カテゴリーと掲示板がコミュニティを運営する人の
// 決めるものであることと、開発者が掲示板へslugで辿り着くことによります。実行のたびに
// 名前が変われば、ブラウザの履歴に残ったアドレスは何も指さなくなります。
//
// 掲示板をそれを並べるカテゴリーの下に入れ子にせず1つの平坦な一覧にしているのは、それが
// サイドバーの描く形であり (ADR 0011)、どのカテゴリーにも属さない掲示板には入れ子になる
// 先が無いためです。ここでの順序はサイドバーが見せる順序であり、各行のpositionはこれを
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

// coldStartBoardsはcold-startプロファイルが生成するコミュニティです。
// インスタンスが開くときに持つ掲示板1つだけで、どのカテゴリーにも属しません。
// コミュニティはすべてを受け止める掲示板から始まり、カテゴリーはまとめる対象の掲示板が
// できてから描かれます。どちらも先回りして用意すれば、作った人以外に開く理由の無い空の
// 器が残ります (ADR 0010・ADR 0011)。
//
// この掲示板は成熟したコミュニティの賑わう掲示板と同じslugを持ちます。片方の状態を
// 眺めながら書き留めたアドレスが、もう片方でも掲示板を開くようにするためです。
var coldStartBoards = []seedBoard{
	{
		slug:        "chat",
		name:        "雑談",
		description: "話題を決めずに書き込む場所です。",
		activity:    boardQuiet,
	},
}

// seededBoardは実行が作成した掲示板です。それを埋める生成器がどれだけ書くのかを
// 知れるよう、記述に使った賑わいを一緒に運びます。
type seededBoard struct {
	board    *model.Board
	activity boardActivity
}

// generateBoardsはカテゴリーと、コミュニティが提供する掲示板を作成します。
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

// validateSeedSlugsは書き下したslugを、ここに置いた写しではなくアプリケーションが
// すべてのカテゴリーと掲示板に課している規則で検査します。本ファイルが持ち込むslugが
// /c/{slug} と /b/{slug} で指せるものであるようにするためです。templates.BoardPathは
// slugをそのままパスへ置くため、パスやクエリの文字を含むslugは掲示板ではないどこかを
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
