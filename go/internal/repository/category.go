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

// CategoryRepositoryはsqlc生成のクエリ経由でcategoriesを読み書きします。
type CategoryRepository struct {
	reader *query.Queries
	writer *query.Queries
}

// NewCategoryRepositoryは、データベースの読み取り用プールで読み、書き込み用
// プールで書くCategoryRepositoryを生成します。
func NewCategoryRepository(db *database.DB) *CategoryRepository {
	return &CategoryRepository{reader: query.New(db.Reader), writer: query.New(db.Writer)}
}

// WithTxはqueriesをtx内で実行する新しいCategoryRepositoryを返し、UseCaseが
// 本リポジトリを自身のトランザクションに参加させられるようにします。レシーバ自身は
// 変更しません。
func (r *CategoryRepository) WithTx(tx *sql.Tx) *CategoryRepository {
	q := r.writer.WithTx(tx)
	return &CategoryRepository{reader: q, writer: q}
}

// FindByIDは指定idのカテゴリーを返し、存在しない場合は (nil, nil) を返します。
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

// FindBySlugは指定slugのカテゴリーを返し、存在しない場合は (nil, nil) を
// 返します。slug列はNOCASE照合のため大文字小文字を無視します (slugのUNIQUE制約が
// 強制するのと同じ大小の規則)。未存在は正常なルックアップ結果であり — /c/{slug} が
// 404を返すと判断する手立てです — エラーではありません。
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

// CreateCategoryInputはカテゴリーの作成に必要な属性を保持します。idと
// タイムスタンプはDB側で採番されます。
type CreateCategoryInput struct {
	Slug     string
	Name     string
	Position int
}

// Createはカテゴリーを挿入し、DBが採番したidとタイムスタンプを設定した状態で
// 返します。
//
// 挿入の前に、slugが /c/{slug} の前提としている規則に合うことを検査します。列は
// NOCASE照合であり1つのカテゴリーを2通りの綴りで持つことはできませんが、その
// 唯一の綴りを小文字に保つ仕組みはスキーマにありません。そしてページは大文字小文字の
// 異なるリクエストを、保存されている綴りへリダイレクトします。したがって大文字を含む
// slugを入れると、大文字のURLのほうが正規になってしまいます。呼び出し側ではなく
// ここで検査するのは、カテゴリーを作るすべての経路を、後から増えるものも含めて1つの
// 検査で覆うためです。
func (r *CategoryRepository) Create(ctx context.Context, input CreateCategoryInput) (*model.Category, error) {
	if !model.IsValidSlug(input.Slug) {
		return nil, fmt.Errorf("カテゴリーのslugが不正: slug=%q", input.Slug)
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

// toModelはquery.Categoryをmodel.Categoryに変換し、リポジトリの境界で生のidを
// 型付きのCategoryIDに、保存書式の時刻をtime.Timeにキャストします。
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
