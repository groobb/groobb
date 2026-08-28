package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/query"
)

// BoardRepository reads and writes boards through sqlc-generated queries.
//
// [Ja] BoardRepository は sqlc 生成のクエリ経由で boards を読み書きします。
type BoardRepository struct {
	reader *query.Queries
	writer *query.Queries
}

// NewBoardRepository creates a BoardRepository that reads through the database's
// read pool and writes through its write pool.
//
// [Ja] NewBoardRepository は、データベースの読み取り用プールで読み、書き込み用プールで
// 書く BoardRepository を生成します。
func NewBoardRepository(db *database.DB) *BoardRepository {
	return &BoardRepository{reader: query.New(db.Reader), writer: query.New(db.Writer)}
}

// WithTx returns a new BoardRepository whose queries run inside tx, so a UseCase
// can enlist this repository in its transaction. The receiver is left unchanged.
//
// [Ja] WithTx は queries を tx 内で実行する新しい BoardRepository を返し、UseCase が
// 本リポジトリを自身のトランザクションに参加させられるようにします。レシーバ自身は
// 変更しません。
func (r *BoardRepository) WithTx(tx *sql.Tx) *BoardRepository {
	q := r.writer.WithTx(tx)
	return &BoardRepository{reader: q, writer: q}
}

// FindByID returns the board with the given id, or (nil, nil) when none exists.
// A thread names the board it was posted in by id, so a page rendering a thread
// resolves its board through this rather than through the slug /b/{slug}
// carries. Absence is a normal lookup outcome, not an error.
//
// [Ja] FindByID は指定 id の掲示板を返し、存在しない場合は (nil, nil) を返します。
// スレッドは自身が立った掲示板を id で名指すため、スレッドを描画するページは /b/{slug} が
// 運ぶ slug ではなくこれで掲示板を解決します。未存在は正常なルックアップ結果であり
// エラーではありません。
func (r *BoardRepository) FindByID(ctx context.Context, id model.BoardID) (*model.Board, error) {
	row, err := r.reader.GetBoardByID(ctx, int64(id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return r.toModel(row), nil
}

// FindBySlug returns the board with the given slug, or (nil, nil) when none
// exists. The slug column collates NOCASE, so the match ignores letter case (the
// same casing rule the slug UNIQUE constraint enforces). The lookup takes the
// slug alone because /b/{slug} names a board without naming its category.
// Absence is a normal lookup outcome, not an error.
//
// [Ja] FindBySlug は指定 slug の掲示板を返し、存在しない場合は (nil, nil) を返します。
// slug 列は NOCASE 照合のため大文字小文字を無視します (slug の UNIQUE 制約が強制するのと
// 同じ大小の規則)。ルックアップが slug だけを取るのは、/b/{slug} が掲示板をそのカテゴリーを
// 言わずに名指しするためです。未存在は正常なルックアップ結果でありエラーではありません。
func (r *BoardRepository) FindBySlug(ctx context.Context, slug string) (*model.Board, error) {
	row, err := r.reader.GetBoardBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return r.toModel(row), nil
}

// ListAll returns every board of the community in the order it placed them
// (position ascending, id breaking a tie so equal positions still come back in a
// fixed order). There is no filter and no limit because the sidebar lists the
// community's boards flat rather than under the categories that group them
// (ADR 0011), so leaving any of them out would hide a board from every page of
// the shell.
//
// [Ja] ListAll はコミュニティのすべての掲示板を、コミュニティが並べた順 (position の
// 昇順。position が同じ場合も順序が固定されるよう id で同着を解く) で返します。絞り込みも
// 上限も無いのは、サイドバーがコミュニティの掲示板を、それをまとめるカテゴリーの下では
// なくフラットに並べるためです (ADR 0011)。どれかを落とせば、シェルを持つすべての
// ページからその掲示板が隠れます。
func (r *BoardRepository) ListAll(ctx context.Context) ([]*model.Board, error) {
	rows, err := r.reader.ListBoards(ctx)
	if err != nil {
		return nil, err
	}
	return r.toModels(rows), nil
}

// ListByCategoryID returns the boards the given category lists, in the order the
// community placed them (position ascending, id breaking a tie so equal
// positions still come back in a fixed order). A category the community has yet
// to place a board in yields an empty slice, which is a state its page renders
// rather than a failure.
//
// [Ja] ListByCategoryID は指定したカテゴリーが並べる掲示板を、コミュニティが並べた順
// (position の昇順。position が同じ場合も順序が固定されるよう id で同着を解く) で
// 返します。コミュニティがまだ掲示板を置いていないカテゴリーは空のスライスになります。
// それはそのページが描画する状態であって失敗ではありません。
func (r *BoardRepository) ListByCategoryID(ctx context.Context, categoryID model.CategoryID) ([]*model.Board, error) {
	rows, err := r.reader.ListBoardsByCategoryID(ctx, rawCategoryID(&categoryID))
	if err != nil {
		return nil, err
	}
	return r.toModels(rows), nil
}

// CreateBoardInput holds the attributes needed to create a board. id and the
// timestamps are assigned by the database.
//
// [Ja] CreateBoardInput は掲示板の作成に必要な属性を保持します。id とタイムスタンプは
// DB 側で採番されます。
type CreateBoardInput struct {
	// CategoryID is the category the board is listed under, and nil for a board
	// the community places outside every category (ADR 0011).
	//
	// [Ja] CategoryID は掲示板を並べるカテゴリーで、コミュニティがどのカテゴリーにも
	// 属さない形で置く掲示板では nil です (ADR 0011)。
	CategoryID *model.CategoryID

	Slug        string
	Name        string
	Description string
	Position    int
}

// Create inserts a board and returns it with the database-assigned id and
// timestamps populated.
//
// The slug is checked against the rule /b/{slug} relies on before the insert,
// for the reason CategoryRepository.Create documents: the schema keeps the
// spelling unique but not lowercase, and BoardPath places whatever is stored
// into the path as written.
//
// [Ja] Create は掲示板を挿入し、DB が採番した id とタイムスタンプを設定した状態で
// 返します。
//
// 挿入の前に、slug が /b/{slug} の前提としている規則に合うことを検査します。理由は
// CategoryRepository.Create が記すとおりで、スキーマは綴りを一意には保ちますが小文字に
// は保たず、BoardPath は保存されている綴りをそのままパスへ置きます。
func (r *BoardRepository) Create(ctx context.Context, input CreateBoardInput) (*model.Board, error) {
	if !model.IsValidSlug(input.Slug) {
		return nil, fmt.Errorf("掲示板の slug が不正: slug=%q", input.Slug)
	}

	row, err := r.writer.CreateBoard(ctx, query.CreateBoardParams{
		CategoryID:  rawCategoryID(input.CategoryID),
		Slug:        input.Slug,
		Name:        input.Name,
		Description: input.Description,
		Position:    int64(input.Position),
	})
	if err != nil {
		return nil, err
	}
	return r.toModel(row), nil
}

// toModels converts the rows of a listing into models, keeping the order the
// query returned them in.
//
// [Ja] toModels は一覧のクエリが返した行をモデルへ変換し、クエリが返した順序を保ちます。
func (r *BoardRepository) toModels(rows []query.Board) []*model.Board {
	boards := make([]*model.Board, len(rows))
	for i, row := range rows {
		boards[i] = r.toModel(row)
	}
	return boards
}

// toModel converts a query.Board row into a model.Board, casting the raw ids
// into their typed forms and the stored timestamps back into time.Time at the
// repository boundary.
//
// [Ja] toModel は query.Board を model.Board に変換し、リポジトリの境界で生の id を
// 型付きの形に、保存書式の時刻を time.Time にキャストします。
func (r *BoardRepository) toModel(row query.Board) *model.Board {
	return &model.Board{
		ID:          model.BoardID(row.ID),
		CategoryID:  typedCategoryID(row.CategoryID),
		Slug:        row.Slug,
		Name:        row.Name,
		Description: row.Description,
		Position:    int(row.Position),
		CreatedAt:   time.Time(row.CreatedAt),
		UpdatedAt:   time.Time(row.UpdatedAt),
	}
}

// rawCategoryID converts a board's category on its way into a query, returning
// nil for a board sitting in none.
//
// [Ja] rawCategoryID は掲示板のカテゴリーをクエリへ渡す方向で変換し、どのカテゴリーにも
// 属さない掲示板には nil を返します。
func rawCategoryID(id *model.CategoryID) *int64 {
	if id == nil {
		return nil
	}
	raw := int64(*id)
	return &raw
}

// typedCategoryID converts a board's category on its way out of a query row,
// returning nil for a board sitting in none.
//
// [Ja] typedCategoryID は掲示板のカテゴリーをクエリの行から取り出す方向で変換し、どの
// カテゴリーにも属さない掲示板には nil を返します。
func typedCategoryID(raw *int64) *model.CategoryID {
	if raw == nil {
		return nil
	}
	id := model.CategoryID(*raw)
	return &id
}
