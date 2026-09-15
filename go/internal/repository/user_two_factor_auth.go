package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/query"
	"github.com/groobb/groobb/go/internal/sqlitetime"
)

// UserTwoFactorAuthRepositoryはsqlc生成のクエリ経由でuser_two_factor_authsを
// 読み書きします。
type UserTwoFactorAuthRepository struct {
	reader *query.Queries
	writer *query.Queries
}

// NewUserTwoFactorAuthRepositoryは、データベースの読み取り用プールで読み、書き込み用プールで書く
// UserTwoFactorAuthRepositoryを生成します。
func NewUserTwoFactorAuthRepository(db *database.DB) *UserTwoFactorAuthRepository {
	return &UserTwoFactorAuthRepository{reader: query.New(db.Reader), writer: query.New(db.Writer)}
}

// WithTxはqueriesをtx内で実行する新しいUserTwoFactorAuthRepositoryを返し、
// UseCaseが本リポジトリを自身のトランザクションに参加させられる (例: リカバリーコードの
// 消費とセッション発行をアトミックに行う) ようにします。レシーバ自身は変更しません。
func (r *UserTwoFactorAuthRepository) WithTx(tx *sql.Tx) *UserTwoFactorAuthRepository {
	q := r.writer.WithTx(tx)
	return &UserTwoFactorAuthRepository{reader: q, writer: q}
}

// FindByUserIDは指定IDのユーザーの2FA設定を返し、存在しない場合 (登録を一度も
// 開始していないユーザー) は (nil, nil) を返します。未存在は正常なルックアップ結果であり
// エラーではありません。業務上の失敗として扱うかは呼び出し側が判断します。
func (r *UserTwoFactorAuthRepository) FindByUserID(ctx context.Context, userID model.UserID) (*model.UserTwoFactorAuth, error) {
	row, err := r.reader.GetUserTwoFactorAuthByUserID(ctx, int64(userID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return r.toModel(row)
}

// FindEnabledByUserIDはユーザーの2FA設定が有効な場合のみ返し、設定が無いか
// まだ登録中の場合は (nil, nil) を返します。サインインはTOTPチャレンジを要求するか
// どうかの判定にこれを使うため、未有効化の行は設定なしと同じ扱いになります。
func (r *UserTwoFactorAuthRepository) FindEnabledByUserID(ctx context.Context, userID model.UserID) (*model.UserTwoFactorAuth, error) {
	row, err := r.reader.GetEnabledUserTwoFactorAuthByUserID(ctx, int64(userID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return r.toModel(row)
}

// CreateUserTwoFactorAuthInputはユーザーの2FA登録を開始するために必要な属性を
// 保持します。idとタイムスタンプはDB側で採番され、行は無効かつリカバリーコード無しで
// 始まります。ユーザーがTOTPコードを確認した時点でEnableが有効化します。
type CreateUserTwoFactorAuthInput struct {
	UserID model.UserID
	Secret string
}

// Createは未有効化の2FA設定 (登録用secret) を挿入し、DBが採番したidと
// タイムスタンプを設定した状態で返します。挿入はON CONFLICT (user_id) DO NOTHINGの
// ため、ユーザーの行が既に存在するとき (例: 同時の初回設定リクエストが先に挿入したとき) は
// 何も挿入せず (nil, nil) を返します。呼び出し側はその既存の登録を取り直して再利用します。
// これにより、設定が同時に走ってもuser_idのunique制約に一切違反せずget-or-createを
// 冪等に保ちます。
func (r *UserTwoFactorAuthRepository) Create(ctx context.Context, input CreateUserTwoFactorAuthInput) (*model.UserTwoFactorAuth, error) {
	row, err := r.writer.CreateUserTwoFactorAuth(ctx, query.CreateUserTwoFactorAuthParams{
		UserID: int64(input.UserID),
		Secret: input.Secret,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return r.toModel(row)
}

// Enableはユーザーの未有効化の2FA設定を有効化します。設定をenabledにし、
// enabled_atを打刻し、生成したリカバリーコードを保存する処理を、enabled = falseで
// ガードした1回の更新で行います。実際に行を有効化したかを返します。falseは未有効化の行が
// 一致しなかった (2FAが既に有効 — 例えば同時の有効化が競合に勝った — か、ユーザーが登録して
// いない) ことを意味し、その場合呼び出し側のリカバリーコードは保存されておらず表示しては
// なりません。このガードにより有効化は冪等になり、2つ目の同時有効化が保存済みリカバリー
// コードを上書きするのを防ぎます。
func (r *UserTwoFactorAuthRepository) Enable(ctx context.Context, userID model.UserID, recoveryCodes []string) (bool, error) {
	encoded, err := encodeRecoveryCodes(recoveryCodes)
	if err != nil {
		return false, err
	}

	rows, err := r.writer.EnableUserTwoFactorAuth(ctx, query.EnableUserTwoFactorAuthParams{
		UserID:        int64(userID),
		RecoveryCodes: encoded,
	})
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

// UpdateRecoveryCodesはユーザーの保存済みリカバリーコードを与えられたスライスで
// 置き換えます (updated_atも更新)。リカバリーコードでのサインインが、1つ消費した後に
// 残りのコードを書き戻すために使います。
func (r *UserTwoFactorAuthRepository) UpdateRecoveryCodes(ctx context.Context, userID model.UserID, recoveryCodes []string) error {
	encoded, err := encodeRecoveryCodes(recoveryCodes)
	if err != nil {
		return err
	}

	return r.writer.UpdateUserTwoFactorAuthRecoveryCodes(ctx, query.UpdateUserTwoFactorAuthRecoveryCodesParams{
		UserID:        int64(userID),
		RecoveryCodes: encoded,
	})
}

// Deleteはユーザーの2FA設定を削除し、2段階認証を無効化してsecretと
// リカバリーコードを行ごと破棄します。設定が無いときに削除してもエラーにならないため、
// 無効化は冪等です。
func (r *UserTwoFactorAuthRepository) Delete(ctx context.Context, userID model.UserID) error {
	return r.writer.DeleteUserTwoFactorAuthByUserID(ctx, int64(userID))
}

// toModelはquery.UserTwoFactorAuthをmodel.UserTwoFactorAuthに変換し、
// リポジトリの境界で生のidを型付きIDにキャストし、保存されたリカバリーコードを
// デコードします。保存された値が、その列が保持するはずのJSON配列になっていない場合は
// エラーを返します。
func (r *UserTwoFactorAuthRepository) toModel(row query.UserTwoFactorAuth) (*model.UserTwoFactorAuth, error) {
	recoveryCodes, err := decodeRecoveryCodes(row.RecoveryCodes)
	if err != nil {
		return nil, err
	}

	return &model.UserTwoFactorAuth{
		ID:            model.UserTwoFactorAuthID(row.ID),
		UserID:        model.UserID(row.UserID),
		Secret:        row.Secret,
		Enabled:       row.Enabled,
		EnabledAt:     sqlitetime.TimePtr(row.EnabledAt),
		RecoveryCodes: recoveryCodes,
		CreatedAt:     time.Time(row.CreatedAt),
		UpdatedAt:     time.Time(row.UpdatedAt),
	}, nil
}

// encodeRecoveryCodesはリカバリーコードを、列が保存するJSON配列として表現
// します。SQLiteに配列型は無いため、リストを値に取る列はJSONを保持するTEXTであり、
// スライスがそのテキストになる場所がリポジトリの境界です。
func encodeRecoveryCodes(recoveryCodes []string) (string, error) {
	// nilのスライスは "null" にエンコードされ、列のjson_typeチェックに弾かれる。
	// 空のリストは、列の既定値でもある空配列として書く。
	if recoveryCodes == nil {
		recoveryCodes = []string{}
	}

	encoded, err := json.Marshal(recoveryCodes)
	if err != nil {
		return "", fmt.Errorf("failed to encode the recovery codes: %w", err)
	}
	return string(encoded), nil
}

// decodeRecoveryCodesは列が保存するJSON配列をスライスに解釈し直します。
func decodeRecoveryCodes(encoded string) ([]string, error) {
	var recoveryCodes []string
	if err := json.Unmarshal([]byte(encoded), &recoveryCodes); err != nil {
		return nil, fmt.Errorf("failed to decode the recovery codes: %w", err)
	}
	return recoveryCodes, nil
}
