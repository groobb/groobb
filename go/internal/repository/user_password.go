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

// UserPasswordRepositoryはsqlc生成のクエリ経由でuser_passwordsを読み書き
// します。
type UserPasswordRepository struct {
	reader *query.Queries
	writer *query.Queries
}

// NewUserPasswordRepositoryは、データベースの読み取り用プールで読み、書き込み用プールで書く
// UserPasswordRepositoryを生成します。
func NewUserPasswordRepository(db *database.DB) *UserPasswordRepository {
	return &UserPasswordRepository{reader: query.New(db.Reader), writer: query.New(db.Writer)}
}

// WithTxはqueriesをtx内で実行する新しいUserPasswordRepositoryを返し、
// UseCaseが本リポジトリを自身のトランザクションに参加させられる (例: ユーザーと
// そのパスワードをアトミックに作成する) ようにします。レシーバ自身は変更しません。
func (r *UserPasswordRepository) WithTx(tx *sql.Tx) *UserPasswordRepository {
	q := r.writer.WithTx(tx)
	return &UserPasswordRepository{reader: q, writer: q}
}

// FindByUserIDは指定IDのユーザーのパスワード資格情報を返し、存在しない場合
// (SSOのみのユーザー、またはアカウントが未完成のユーザー) は (nil, nil) を返します。
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

// CreateUserPasswordInputはパスワード資格情報の作成に必要な属性を保持します。
// idとタイムスタンプはDB側で採番されます。PasswordDigestは既にbcryptハッシュで
// ある必要があり、本リポジトリは平文を扱いません。
type CreateUserPasswordInput struct {
	UserID         model.UserID
	PasswordDigest string
}

// Createはパスワード資格情報を挿入し、DBが採番したidとタイムスタンプを
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

// UpdatePasswordDigestはユーザーの保存パスワードハッシュを与えられたダイジェストで
// 置き換えます (updated_atも更新)。パスワードリセットフローが既存ユーザーに新しい資格情報を
// 設定するために使います。本リポジトリは平文を扱わないため、ダイジェストは既にbcrypt
// ハッシュである必要があります。1行も更新しなかった場合 (ユーザーがまだ資格情報を持たない)
// もここではエラーとしません。呼び出し側が事前にユーザーを解決するため、素のUPDATEに
// 留めます。
func (r *UserPasswordRepository) UpdatePasswordDigest(ctx context.Context, userID model.UserID, passwordDigest string) error {
	return r.writer.UpdateUserPasswordDigestByUserID(ctx, query.UpdateUserPasswordDigestByUserIDParams{
		UserID:         int64(userID),
		PasswordDigest: passwordDigest,
	})
}

// toModelはquery.UserPasswordをmodel.UserPasswordに変換し、リポジトリの
// 境界で生のidを型付きIDにキャストします。
func (r *UserPasswordRepository) toModel(row query.UserPassword) *model.UserPassword {
	return &model.UserPassword{
		ID:             model.UserPasswordID(row.ID),
		UserID:         model.UserID(row.UserID),
		PasswordDigest: row.PasswordDigest,
		CreatedAt:      time.Time(row.CreatedAt),
		UpdatedAt:      time.Time(row.UpdatedAt),
	}
}
