package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/query"
)

// UserPasswordRepository reads and writes user_passwords through sqlc-generated
// queries.
//
// [Ja] UserPasswordRepository は sqlc 生成のクエリ経由で user_passwords を読み書き
// します。
type UserPasswordRepository struct {
	reader *query.Queries
	writer *query.Queries
}

// NewUserPasswordRepository creates a UserPasswordRepository that reads through the database's read pool
// and writes through its write pool.
//
// [Ja] NewUserPasswordRepository は、データベースの読み取り用プールで読み、書き込み用プールで書く
// UserPasswordRepository を生成します。
func NewUserPasswordRepository(db *database.DB) *UserPasswordRepository {
	return &UserPasswordRepository{reader: query.New(db.Reader), writer: query.New(db.Writer)}
}

// WithTx returns a new UserPasswordRepository whose queries run inside tx, so a
// UseCase can enlist this repository in its transaction (e.g. to create the user
// and its password atomically). The receiver is left unchanged.
//
// [Ja] WithTx は queries を tx 内で実行する新しい UserPasswordRepository を返し、
// UseCase が本リポジトリを自身のトランザクションに参加させられる (例: ユーザーと
// そのパスワードをアトミックに作成する) ようにします。レシーバ自身は変更しません。
func (r *UserPasswordRepository) WithTx(tx *sql.Tx) *UserPasswordRepository {
	q := r.writer.WithTx(tx)
	return &UserPasswordRepository{reader: q, writer: q}
}

// FindByUserID returns the password credential of the user with the given ID, or
// (nil, nil) when none exists (an SSO-only user, or one whose account is not yet
// fully created). Absence is a normal lookup outcome, not an error; the caller
// decides whether to treat it as a business-level failure.
//
// [Ja] FindByUserID は指定 ID のユーザーのパスワード資格情報を返し、存在しない場合
// (SSO のみのユーザー、またはアカウントが未完成のユーザー) は (nil, nil) を返します。
// 未存在は正常なルックアップ結果でありエラーではありません。業務上の失敗として扱うかは
// 呼び出し側が判断します。
func (r *UserPasswordRepository) FindByUserID(ctx context.Context, userID model.UserID) (*model.UserPassword, error) {
	row, err := r.reader.GetUserPasswordByUserID(ctx, int64(userID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return r.toModel(row), nil
}

// CreateUserPasswordInput holds the attributes needed to create a password
// credential. id and the timestamps are assigned by the database. PasswordDigest
// must already be a bcrypt hash; this repository never sees the plaintext.
//
// [Ja] CreateUserPasswordInput はパスワード資格情報の作成に必要な属性を保持します。
// id とタイムスタンプは DB 側で採番されます。PasswordDigest は既に bcrypt ハッシュで
// ある必要があり、本リポジトリは平文を扱いません。
type CreateUserPasswordInput struct {
	UserID         model.UserID
	PasswordDigest string
}

// Create inserts a password credential and returns it with the database-assigned
// id and timestamps populated.
//
// [Ja] Create はパスワード資格情報を挿入し、DB が採番した id とタイムスタンプを
// 設定した状態で返します。
func (r *UserPasswordRepository) Create(ctx context.Context, input CreateUserPasswordInput) (*model.UserPassword, error) {
	row, err := r.writer.CreateUserPassword(ctx, query.CreateUserPasswordParams{
		UserID:         int64(input.UserID),
		PasswordDigest: input.PasswordDigest,
	})
	if err != nil {
		return nil, err
	}
	return r.toModel(row), nil
}

// UpdatePasswordDigest replaces the user's stored password hash with the given
// digest (and bumps updated_at). It is used by the password reset flow to set a
// new credential for an existing user; the digest must already be a bcrypt hash,
// as this repository never sees the plaintext. Updating no row (the user has no
// credential yet) is not reported as an error here; the caller resolves the user
// before calling, so this stays a plain UPDATE.
//
// [Ja] UpdatePasswordDigest はユーザーの保存パスワードハッシュを与えられたダイジェストで
// 置き換えます (updated_at も更新)。パスワードリセットフローが既存ユーザーに新しい資格情報を
// 設定するために使います。本リポジトリは平文を扱わないため、ダイジェストは既に bcrypt
// ハッシュである必要があります。1 行も更新しなかった場合 (ユーザーがまだ資格情報を持たない)
// もここではエラーとしません。呼び出し側が事前にユーザーを解決するため、素の UPDATE に
// 留めます。
func (r *UserPasswordRepository) UpdatePasswordDigest(ctx context.Context, userID model.UserID, passwordDigest string) error {
	return r.writer.UpdateUserPasswordDigestByUserID(ctx, query.UpdateUserPasswordDigestByUserIDParams{
		UserID:         int64(userID),
		PasswordDigest: passwordDigest,
	})
}

// toModel converts a query.UserPassword row into a model.UserPassword, casting
// the raw ids into the typed IDs at the repository boundary.
//
// [Ja] toModel は query.UserPassword を model.UserPassword に変換し、リポジトリの
// 境界で生の id を型付き ID にキャストします。
func (r *UserPasswordRepository) toModel(row query.UserPassword) *model.UserPassword {
	return &model.UserPassword{
		ID:             model.UserPasswordID(row.ID),
		UserID:         model.UserID(row.UserID),
		PasswordDigest: row.PasswordDigest,
		CreatedAt:      time.Time(row.CreatedAt),
		UpdatedAt:      time.Time(row.UpdatedAt),
	}
}
