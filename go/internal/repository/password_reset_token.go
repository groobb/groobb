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

// PasswordResetTokenRepository reads and writes password_reset_tokens through
// sqlc-generated queries.
//
// [Ja] PasswordResetTokenRepository は sqlc 生成のクエリ経由で password_reset_tokens を
// 読み書きします。
type PasswordResetTokenRepository struct {
	reader *query.Queries
	writer *query.Queries
}

// NewPasswordResetTokenRepository creates a PasswordResetTokenRepository that reads through the database's read pool
// and writes through its write pool.
//
// [Ja] NewPasswordResetTokenRepository は、データベースの読み取り用プールで読み、書き込み用プールで書く
// PasswordResetTokenRepository を生成します。
func NewPasswordResetTokenRepository(db *database.DB) *PasswordResetTokenRepository {
	return &PasswordResetTokenRepository{reader: query.New(db.Reader), writer: query.New(db.Writer)}
}

// WithTx returns a new PasswordResetTokenRepository whose queries run inside tx,
// so a UseCase can enlist this repository in its transaction (e.g. to delete a
// user's outstanding tokens and create a fresh one atomically). The receiver is
// left unchanged.
//
// [Ja] WithTx は queries を tx 内で実行する新しい PasswordResetTokenRepository を返し、
// UseCase が本リポジトリを自身のトランザクションに参加させられる (例: ユーザーの未使用
// トークンを削除し新しいトークンをアトミックに作成する) ようにします。レシーバ自身は
// 変更しません。
func (r *PasswordResetTokenRepository) WithTx(tx *sql.Tx) *PasswordResetTokenRepository {
	q := r.writer.WithTx(tx)
	return &PasswordResetTokenRepository{reader: q, writer: q}
}

// CreatePasswordResetTokenInput holds the attributes needed to create a reset
// token. id, used_at, and the timestamps are assigned by the database (used_at
// starts NULL). TokenDigest must already be the hash of the token; this
// repository never sees the plaintext.
//
// [Ja] CreatePasswordResetTokenInput はリセットトークンの作成に必要な属性を保持します。
// id / used_at / タイムスタンプは DB 側で採番されます (used_at は NULL で始まります)。
// TokenDigest は既にトークンのハッシュである必要があり、本リポジトリは平文を扱いません。
type CreatePasswordResetTokenInput struct {
	UserID      model.UserID
	TokenDigest string
	ExpiresAt   time.Time
}

// Create inserts a reset token and returns it with the database-assigned id and
// timestamps populated.
//
// [Ja] Create はリセットトークンを挿入し、DB が採番した id とタイムスタンプを設定した
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

// FindByTokenDigest returns the reset token whose stored hash matches the given
// digest, or (nil, nil) when none exists. The lookup is by digest (not the
// plaintext, which is never stored), and token_digest is UNIQUE so it resolves to
// at most one row. Absence is a normal outcome (an unknown or already-deleted
// token), not an error; whether the matched token is usable (unused, unexpired)
// is judged by the caller via the model's IsUsed / IsExpired so the consuming
// flow can report why it failed.
//
// [Ja] FindByTokenDigest は保存ハッシュが与えられたダイジェストに一致するリセット
// トークンを返し、存在しなければ (nil, nil) を返します。ルックアップは (決して保存しない
// 平文ではなく) ダイジェストで行い、token_digest は UNIQUE のため高々 1 行に解決します。
// 未存在は正常な結果 (未知または削除済みのトークン) でありエラーではありません。一致した
// トークンが使えるか (未使用・未期限切れ) は、消費フローが失敗理由を報告できるよう、
// モデルの IsUsed / IsExpired で呼び出し側が判定します。
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

// MarkAsUsed stamps the token's used_at (and updated_at) so it cannot be replayed:
// once spent, FindByTokenDigest still returns it but IsUsed reports true, and the
// password-update flow rejects it. It is called inside the same transaction as the
// password update, so a link is marked used exactly when the new password is set.
//
// [Ja] MarkAsUsed はトークンの used_at (と updated_at) を打刻し、再利用できないように
// します。一度消費すると FindByTokenDigest はなお返しますが IsUsed が true を返し、
// パスワード更新フローが拒否します。パスワード更新と同一トランザクション内で呼ばれるため、
// リンクは新しいパスワードが設定されるのとちょうど同時に使用済みになります。
func (r *PasswordResetTokenRepository) MarkAsUsed(ctx context.Context, id model.PasswordResetTokenID) error {
	return r.writer.MarkPasswordResetTokenAsUsed(ctx, int64(id))
}

// DeleteUnusedByUserID deletes the user's still-unused reset tokens (used_at IS
// NULL), so issuing a fresh token invalidates any earlier outstanding link. Used
// (spent) tokens are left in place as a record of past resets. Deleting nothing
// is not an error.
//
// [Ja] DeleteUnusedByUserID はユーザーのまだ未使用のリセットトークン (used_at IS NULL) を
// 削除し、新しいトークンの発行で以前の未使用リンクを無効化します。使用済み (消費済み) の
// トークンは過去のリセットの記録として残します。何も削除されなくてもエラーではありません。
func (r *PasswordResetTokenRepository) DeleteUnusedByUserID(ctx context.Context, userID model.UserID) error {
	return r.writer.DeleteUnusedPasswordResetTokensByUserID(ctx, int64(userID))
}

// toModel converts a query.PasswordResetToken row into a
// model.PasswordResetToken, casting the raw ids into the typed IDs at the
// repository boundary.
//
// [Ja] toModel は query.PasswordResetToken を model.PasswordResetToken に変換し、
// リポジトリの境界で生の id を型付き ID にキャストします。
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
