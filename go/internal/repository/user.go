// Package repository adapts sqlc-generated queries into domain models. Each
// repository owns one model (model.User <-> UserRepository) and converts query
// rows into that model, keeping the database details out of the upper layers.
//
// [Ja] repository パッケージは sqlc 生成のクエリをドメインモデルに変換します。
// 各リポジトリは 1 つのモデルを担当し (model.User <-> UserRepository)、クエリ結果を
// そのモデルに変換することで、DB の詳細を上位層から隠します。
package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/query"
)

// UserRepository reads and writes users through sqlc-generated queries.
// [Ja] UserRepository は sqlc 生成のクエリ経由で users を読み書きします。
type UserRepository struct {
	q *query.Queries
}

// NewUserRepository creates a UserRepository backed by the given queries.
// [Ja] NewUserRepository は与えられた queries を使う UserRepository を生成します。
func NewUserRepository(q *query.Queries) *UserRepository {
	return &UserRepository{q: q}
}

// WithTx returns a new UserRepository whose queries run inside tx, so a UseCase
// can enlist this repository in its transaction. The receiver is left unchanged.
//
// [Ja] WithTx は queries を tx 内で実行する新しい UserRepository を返し、UseCase が
// 本リポジトリを自身のトランザクションに参加させられるようにします。レシーバ自身は
// 変更しません。
func (r *UserRepository) WithTx(tx pgx.Tx) *UserRepository {
	return &UserRepository{q: r.q.WithTx(tx)}
}

// FindByID returns the user with the given ID, or (nil, nil) when none exists.
// Absence is a normal lookup outcome, not an error; the caller decides whether
// to treat it as a business-level failure.
//
// [Ja] FindByID は指定 ID のユーザーを返し、存在しない場合は (nil, nil) を返します。
// 未存在は正常なルックアップ結果でありエラーではありません。業務上の失敗として扱うか
// は呼び出し側が判断します。
func (r *UserRepository) FindByID(ctx context.Context, id model.UserID) (*model.User, error) {
	row, err := r.q.GetUserByID(ctx, uuid.UUID(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return r.toModel(row), nil
}

// FindByEmail returns the user with the given email, or (nil, nil) when none
// exists. The email column is citext, so the match ignores letter case.
//
// [Ja] FindByEmail は指定 email のユーザーを返し、存在しない場合は (nil, nil) を
// 返します。email 列は citext のため、照合は大文字小文字を無視します。
func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	row, err := r.q.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return r.toModel(row), nil
}

// FindBySessionToken returns the user that owns the session with the given
// token, or (nil, nil) when no session matches the token (an unknown, stale, or
// forged cookie). It resolves the session and its user in a single JOIN so the
// authentication hot path does not pay two round-trips per request.
//
// [Ja] FindBySessionToken は指定 token のセッションを所有するユーザーを返し、token に
// 一致するセッションが無い場合 (未知 / 失効 / 偽造された Cookie) は (nil, nil) を
// 返します。セッションとそのユーザーを 1 度の JOIN で解決し、認証のホットパスが
// リクエストごとに 2 往復しないようにします。
func (r *UserRepository) FindBySessionToken(ctx context.Context, token string) (*model.User, error) {
	row, err := r.q.GetUserBySessionToken(ctx, token)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return r.toModel(row), nil
}

// CreateUserInput holds the identity-level attributes needed to create a user.
// id and the timestamps are assigned by the database.
//
// [Ja] CreateUserInput はユーザー作成に必要な身元レベルの属性を保持します。
// id とタイムスタンプは DB 側で採番されます。
type CreateUserInput struct {
	Email    string
	Locale   string
	TimeZone string
}

// Create inserts a user and returns it with the database-assigned id and
// timestamps populated.
//
// [Ja] Create はユーザーを挿入し、DB が採番した id とタイムスタンプを設定した状態で
// 返します。
func (r *UserRepository) Create(ctx context.Context, input CreateUserInput) (*model.User, error) {
	row, err := r.q.CreateUser(ctx, query.CreateUserParams{
		Email:    input.Email,
		Locale:   input.Locale,
		TimeZone: input.TimeZone,
	})
	if err != nil {
		return nil, err
	}
	return r.toModel(row), nil
}

// toModel converts a query.User row into a model.User, casting the raw uuid into
// the typed UserID at the repository boundary.
//
// [Ja] toModel は query.User を model.User に変換し、リポジトリの境界で生の uuid を
// 型付きの UserID にキャストします。
func (r *UserRepository) toModel(row query.User) *model.User {
	return &model.User{
		ID:        model.UserID(row.ID),
		Email:     row.Email,
		Locale:    row.Locale,
		TimeZone:  row.TimeZone,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}
}
