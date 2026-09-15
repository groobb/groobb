package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/query"
	"github.com/groobb/groobb/go/internal/sqlitetime"
)

// PasswordResetTokenRepositoryはsqlc生成のクエリ経由でpassword_reset_tokensを
// 読み書きします。
type PasswordResetTokenRepository struct {
	reader *query.Queries
	writer *query.Queries
}

// NewPasswordResetTokenRepositoryは、データベースの読み取り用プールで読み、書き込み用プールで書く
// PasswordResetTokenRepositoryを生成します。
func NewPasswordResetTokenRepository(db *database.DB) *PasswordResetTokenRepository {
	return &PasswordResetTokenRepository{reader: query.New(db.Reader), writer: query.New(db.Writer)}
}

// WithTxはqueriesをtx内で実行する新しいPasswordResetTokenRepositoryを返し、
// UseCaseが本リポジトリを自身のトランザクションに参加させられる (例: ユーザーの未使用
// トークンを削除し新しいトークンをアトミックに作成する) ようにします。レシーバ自身は
// 変更しません。
func (r *PasswordResetTokenRepository) WithTx(tx *sql.Tx) *PasswordResetTokenRepository {
	q := r.writer.WithTx(tx)
	return &PasswordResetTokenRepository{reader: q, writer: q}
}

// CreatePasswordResetTokenInputはリセットトークンの作成に必要な属性を保持します。
// id / used_at / タイムスタンプはDB側で採番されます (used_atはNULLで始まります)。
// TokenDigestは既にトークンのハッシュである必要があり、本リポジトリは平文を扱いません。
type CreatePasswordResetTokenInput struct {
	UserID      model.UserID
	TokenDigest string
	ExpiresAt   time.Time
}

// Createはリセットトークンを挿入し、DBが採番したidとタイムスタンプを設定した
// 状態で返します。
func (r *PasswordResetTokenRepository) Create(ctx context.Context, input CreatePasswordResetTokenInput) (*model.PasswordResetToken, error) {
	row, err := r.writer.CreatePasswordResetToken(ctx, query.CreatePasswordResetTokenParams{
		UserID:      int64(input.UserID),
		TokenDigest: input.TokenDigest,
		ExpiresAt:   sqlitetime.Time(input.ExpiresAt),
	})
	if err != nil {
		return nil, err
	}
	return r.toModel(row), nil
}

// FindByTokenDigestは保存ハッシュが与えられたダイジェストに一致するリセット
// トークンを返し、存在しなければ (nil, nil) を返します。ルックアップは (決して保存しない
// 平文ではなく) ダイジェストで行い、token_digestはUNIQUEのため高々1行に解決します。
// 未存在は正常な結果 (未知または削除済みのトークン) でありエラーではありません。一致した
// トークンが使えるか (未使用・未期限切れ) は、消費フローが失敗理由を報告できるよう、
// モデルのIsUsed / IsExpiredで呼び出し側が判定します。
func (r *PasswordResetTokenRepository) FindByTokenDigest(ctx context.Context, tokenDigest string) (*model.PasswordResetToken, error) {
	row, err := r.reader.GetPasswordResetTokenByDigest(ctx, tokenDigest)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return r.toModel(row), nil
}

// MarkAsUsedはトークンのused_at (とupdated_at) を打刻し、再利用できないように
// します。一度消費するとFindByTokenDigestはなお返しますがIsUsedがtrueを返し、
// パスワード更新フローが拒否します。パスワード更新と同一トランザクション内で呼ばれるため、
// リンクは新しいパスワードが設定されるのとちょうど同時に使用済みになります。
func (r *PasswordResetTokenRepository) MarkAsUsed(ctx context.Context, id model.PasswordResetTokenID) error {
	return r.writer.MarkPasswordResetTokenAsUsed(ctx, int64(id))
}

// DeleteUnusedByUserIDはユーザーのまだ未使用のリセットトークン (used_at IS NULL) を
// 削除し、新しいトークンの発行で以前の未使用リンクを無効化します。使用済み (消費済み) の
// トークンは過去のリセットの記録として残します。何も削除されなくてもエラーではありません。
func (r *PasswordResetTokenRepository) DeleteUnusedByUserID(ctx context.Context, userID model.UserID) error {
	return r.writer.DeleteUnusedPasswordResetTokensByUserID(ctx, int64(userID))
}

// toModelはquery.PasswordResetTokenをmodel.PasswordResetTokenに変換し、
// リポジトリの境界で生のidを型付きIDにキャストします。
func (r *PasswordResetTokenRepository) toModel(row query.PasswordResetToken) *model.PasswordResetToken {
	return &model.PasswordResetToken{
		ID:          model.PasswordResetTokenID(row.ID),
		UserID:      model.UserID(row.UserID),
		TokenDigest: row.TokenDigest,
		ExpiresAt:   time.Time(row.ExpiresAt),
		UsedAt:      sqlitetime.TimePtr(row.UsedAt),
		CreatedAt:   time.Time(row.CreatedAt),
		UpdatedAt:   time.Time(row.UpdatedAt),
	}
}
