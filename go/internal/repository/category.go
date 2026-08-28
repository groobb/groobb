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

// CategoryRepository reads and writes categories through sqlc-generated queries.
//
// [Ja] CategoryRepository は sqlc 生成のクエリ経由で categories を読み書きします。
type CategoryRepository struct {
	reader *query.Queries
	writer *query.Queries
}

// NewCategoryRepository creates a CategoryRepository that reads through the
// database's read pool and writes through its write pool.
//
// [Ja] NewCategoryRepository は、データベースの読み取り用プールで読み、書き込み用
// プールで書く CategoryRepository を生成します。
func NewCategoryRepository(db *database.DB) *CategoryRepository {
	return &CategoryRepository{reader: query.New(db.Reader), writer: query.New(db.Writer)}
}

// WithTx returns a new CategoryRepository whose queries run inside tx, so a
// UseCase can enlist this repository in its transaction. The receiver is left
// unchanged.
//
// [Ja] WithTx は queries を tx 内で実行する新しい CategoryRepository を返し、UseCase が
// 本リポジトリを自身のトランザクションに参加させられるようにします。レシーバ自身は
// 変更しません。
func (r *CategoryRepository) WithTx(tx *sql.Tx) *CategoryRepository {
	q := r.writer.WithTx(tx)
	return &CategoryRepository{reader: q, writer: q}
}

// FindByID returns the category with the given id, or (nil, nil) when none
// exists. It is how a caller holding a board resolves the category that lists
// it, which /b/{slug} needs to say where the board sits. Absence is a normal
// lookup outcome, not an error: a caller that holds a foreign key to a category
// is the one that knows a missing row means its own data is inconsistent.
//
// [Ja] FindByID は指定 id のカテゴリーを返し、存在しない場合は (nil, nil) を返します。
// 掲示板を手にした呼び出し元が、それを並べるカテゴリーを解決する手立てであり、
// /b/{slug} が掲示板の在り処を述べるために必要とするものです。未存在は正常な
// ルックアップ結果でありエラーではありません。カテゴリーへの外部キーを持つ呼び出し元
// こそが、行が無いことは自身のデータの不整合を意味すると判断できる立場にあるためです。
func (r *CategoryRepository) FindByID(ctx context.Context, id model.CategoryID) (*model.Category, error) {
	row, err := r.reader.GetCategoryByID(ctx, int64(id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return r.toModel(row), nil
}

// FindBySlug returns the category with the given slug, or (nil, nil) when none
// exists. The slug column collates NOCASE, so the match ignores letter case (the
// same casing rule the slug UNIQUE constraint enforces). Absence is a normal
// lookup outcome — it is how /c/{slug} learns to answer 404 — not an error.
//
// [Ja] FindBySlug は指定 slug のカテゴリーを返し、存在しない場合は (nil, nil) を
// 返します。slug 列は NOCASE 照合のため大文字小文字を無視します (slug の UNIQUE 制約が
// 強制するのと同じ大小の規則)。未存在は正常なルックアップ結果であり — /c/{slug} が
// 404 を返すと判断する手立てです — エラーではありません。
func (r *CategoryRepository) FindBySlug(ctx context.Context, slug string) (*model.Category, error) {
	row, err := r.reader.GetCategoryBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return r.toModel(row), nil
}

// CreateCategoryInput holds the attributes needed to create a category. id and
// the timestamps are assigned by the database.
//
// [Ja] CreateCategoryInput はカテゴリーの作成に必要な属性を保持します。id と
// タイムスタンプは DB 側で採番されます。
type CreateCategoryInput struct {
	Slug     string
	Name     string
	Position int
}

// Create inserts a category and returns it with the database-assigned id and
// timestamps populated.
//
// The slug is checked against the rule /c/{slug} relies on before the insert.
// The column collates NOCASE and so cannot hold a category twice under two
// spellings, but nothing in the schema keeps the one stored spelling lowercase,
// and the page redirects a differently cased request to whatever is stored. A
// slug entered with a capital would therefore make the uppercase URL the
// canonical one. Checking here rather than in the caller means every path that
// creates a category is covered by the one check, including any added after
// this one.
//
// [Ja] Create はカテゴリーを挿入し、DB が採番した id とタイムスタンプを設定した状態で
// 返します。
//
// 挿入の前に、slug が /c/{slug} の前提としている規則に合うことを検査します。列は
// NOCASE 照合であり 1 つのカテゴリーを 2 通りの綴りで持つことはできませんが、その
// 唯一の綴りを小文字に保つ仕組みはスキーマにありません。そしてページは大文字小文字の
// 異なるリクエストを、保存されている綴りへリダイレクトします。したがって大文字を含む
// slug を入れると、大文字の URL のほうが正規になってしまいます。呼び出し側ではなく
// ここで検査するのは、カテゴリーを作るすべての経路を、後から増えるものも含めて 1 つの
// 検査で覆うためです。
func (r *CategoryRepository) Create(ctx context.Context, input CreateCategoryInput) (*model.Category, error) {
	if !model.IsValidSlug(input.Slug) {
		return nil, fmt.Errorf("カテゴリーの slug が不正: slug=%q", input.Slug)
	}

	row, err := r.writer.CreateCategory(ctx, query.CreateCategoryParams{
		Slug:     input.Slug,
		Name:     input.Name,
		Position: int64(input.Position),
	})
	if err != nil {
		return nil, err
	}
	return r.toModel(row), nil
}

// toModel converts a query.Category row into a model.Category, casting the raw
// id into the typed CategoryID and the stored timestamps back into time.Time at
// the repository boundary.
//
// [Ja] toModel は query.Category を model.Category に変換し、リポジトリの境界で生の id を
// 型付きの CategoryID に、保存書式の時刻を time.Time にキャストします。
func (r *CategoryRepository) toModel(row query.Category) *model.Category {
	return &model.Category{
		ID:        model.CategoryID(row.ID),
		Slug:      row.Slug,
		Name:      row.Name,
		Position:  int(row.Position),
		CreatedAt: time.Time(row.CreatedAt),
		UpdatedAt: time.Time(row.UpdatedAt),
	}
}
