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

// BoardRepositoryはsqlc生成のクエリ経由でboardsを読み書きします。
type BoardRepository struct {
	reader *query.Queries
	writer *query.Queries
}

// NewBoardRepositoryは、データベースの読み取り用プールで読み、書き込み用プールで
// 書くBoardRepositoryを生成します。
func NewBoardRepository(db *database.DB) *BoardRepository {
	return &BoardRepository{reader: query.New(db.Reader), writer: query.New(db.Writer)}
}

// WithTxはqueriesをtx内で実行する新しいBoardRepositoryを返し、UseCaseが
// 本リポジトリを自身のトランザクションに参加させられるようにします。レシーバ自身は
// 変更しません。
func (r *BoardRepository) WithTx(tx *sql.Tx) *BoardRepository {
	q := r.writer.WithTx(tx)
	return &BoardRepository{reader: q, writer: q}
}

// FindByIDは指定idの掲示板を返し、存在しない場合は (nil, nil) を返します。
// スレッドは自身が立った掲示板をidで名指すため、スレッドを描画するページは /b/{slug} が
// 運ぶslugではなくこれで掲示板を解決します。未存在は正常なルックアップ結果であり
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

// FindBySlugは指定slugの掲示板を返し、存在しない場合は (nil, nil) を返します。
// slug列はNOCASE照合のため大文字小文字を無視します (slugのUNIQUE制約が強制するのと
// 同じ大小の規則)。ルックアップがslugだけを取るのは、/b/{slug} が掲示板をそのカテゴリーを
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

// ListAllはコミュニティのすべての掲示板を、コミュニティが並べた順 (positionの
// 昇順。positionが同じ場合も順序が固定されるようidで同着を解く) で返します。絞り込みも
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

// ListByCategoryIDは指定したカテゴリーが並べる掲示板を、コミュニティが並べた順
// (positionの昇順。positionが同じ場合も順序が固定されるようidで同着を解く) で
// 返します。コミュニティがまだ掲示板を置いていないカテゴリーは空のスライスになります。
// それはそのページが描画する状態であって失敗ではありません。
func (r *BoardRepository) ListByCategoryID(ctx context.Context, categoryID model.CategoryID) ([]*model.Board, error) {
	rows, err := r.reader.ListBoardsByCategoryID(ctx, rawCategoryID(&categoryID))
	if err != nil {
		return nil, err
	}
	return r.toModels(rows), nil
}

// CreateBoardInputは掲示板の作成に必要な属性を保持します。idとタイムスタンプは
// DB側で採番されます。
type CreateBoardInput struct {
	// CategoryIDは掲示板を並べるカテゴリーで、コミュニティがどのカテゴリーにも
	// 属さない形で置く掲示板ではnilです (ADR 0011)。
	CategoryID *model.CategoryID

	Slug        string
	Name        string
	Description string
	Position    int
}

// Createは掲示板を挿入し、DBが採番したidとタイムスタンプを設定した状態で
// 返します。
//
// 挿入の前に、slugが /b/{slug} の前提としている規則に合うことを検査します。理由は
// CategoryRepository.Createが記すとおりで、スキーマは綴りを一意には保ちますが小文字に
// は保たず、BoardPathは保存されている綴りをそのままパスへ置きます。
func (r *BoardRepository) Create(ctx context.Context, input CreateBoardInput) (*model.Board, error) {
	if !model.IsValidSlug(input.Slug) {
		return nil, fmt.Errorf("掲示板のslugが不正: slug=%q", input.Slug)
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

// toModelsは一覧のクエリが返した行をモデルへ変換し、クエリが返した順序を保ちます。
func (r *BoardRepository) toModels(rows []query.Board) []*model.Board {
	boards := make([]*model.Board, len(rows))
	for i, row := range rows {
		boards[i] = r.toModel(row)
	}
	return boards
}

// toModelはquery.Boardをmodel.Boardに変換し、リポジトリの境界で生のidを
// 型付きの形に、保存書式の時刻をtime.Timeにキャストします。
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

// rawCategoryIDは掲示板のカテゴリーをクエリへ渡す方向で変換し、どのカテゴリーにも
// 属さない掲示板にはnilを返します。
func rawCategoryID(id *model.CategoryID) *int64 {
	if id == nil {
		return nil
	}
	raw := int64(*id)
	return &raw
}

// typedCategoryIDは掲示板のカテゴリーをクエリの行から取り出す方向で変換し、どの
// カテゴリーにも属さない掲示板にはnilを返します。
func typedCategoryID(raw *int64) *model.CategoryID {
	if raw == nil {
		return nil
	}
	id := model.CategoryID(*raw)
	return &id
}
