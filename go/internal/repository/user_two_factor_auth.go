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

// UserTwoFactorAuthRepository reads and writes user_two_factor_auths through
// sqlc-generated queries.
//
// [Ja] UserTwoFactorAuthRepository は sqlc 生成のクエリ経由で user_two_factor_auths を
// 読み書きします。
type UserTwoFactorAuthRepository struct {
	reader *query.Queries
	writer *query.Queries
}

// NewUserTwoFactorAuthRepository creates a UserTwoFactorAuthRepository that reads through the database's read pool
// and writes through its write pool.
//
// [Ja] NewUserTwoFactorAuthRepository は、データベースの読み取り用プールで読み、書き込み用プールで書く
// UserTwoFactorAuthRepository を生成します。
func NewUserTwoFactorAuthRepository(db *database.DB) *UserTwoFactorAuthRepository {
	return &UserTwoFactorAuthRepository{reader: query.New(db.Reader), writer: query.New(db.Writer)}
}

// WithTx returns a new UserTwoFactorAuthRepository whose queries run inside tx,
// so a UseCase can enlist this repository in its transaction (e.g. to consume a
// recovery code and issue a session atomically). The receiver is left unchanged.
//
// [Ja] WithTx は queries を tx 内で実行する新しい UserTwoFactorAuthRepository を返し、
// UseCase が本リポジトリを自身のトランザクションに参加させられる (例: リカバリーコードの
// 消費とセッション発行をアトミックに行う) ようにします。レシーバ自身は変更しません。
func (r *UserTwoFactorAuthRepository) WithTx(tx *sql.Tx) *UserTwoFactorAuthRepository {
	q := r.writer.WithTx(tx)
	return &UserTwoFactorAuthRepository{reader: q, writer: q}
}

// FindByUserID returns the 2FA setting of the user with the given ID, or
// (nil, nil) when none exists (the user has never started enrolling). Absence is
// a normal lookup outcome, not an error; the caller decides whether to treat it
// as a business-level failure.
//
// [Ja] FindByUserID は指定 ID のユーザーの 2FA 設定を返し、存在しない場合 (登録を一度も
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

// FindEnabledByUserID returns the user's 2FA setting only when it is enabled, or
// (nil, nil) when the user has no setting or is still enrolling. Sign-in uses
// this to decide whether to require a TOTP challenge, so a not-yet-enabled row is
// treated the same as none.
//
// [Ja] FindEnabledByUserID はユーザーの 2FA 設定が有効な場合のみ返し、設定が無いか
// まだ登録中の場合は (nil, nil) を返します。サインインは TOTP チャレンジを要求するか
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

// CreateUserTwoFactorAuthInput holds the attributes needed to start enrolling a
// user in 2FA. id and the timestamps are assigned by the database, and the row
// starts disabled with no recovery codes; Enable activates it once the user
// confirms a TOTP code.
//
// [Ja] CreateUserTwoFactorAuthInput はユーザーの 2FA 登録を開始するために必要な属性を
// 保持します。id とタイムスタンプは DB 側で採番され、行は無効かつリカバリーコード無しで
// 始まります。ユーザーが TOTP コードを確認した時点で Enable が有効化します。
type CreateUserTwoFactorAuthInput struct {
	UserID model.UserID
	Secret string
}

// Create inserts a not-yet-enabled 2FA setting (the enrollment secret) and
// returns it with the database-assigned id and timestamps populated. The insert
// is ON CONFLICT (user_id) DO NOTHING, so when the user already has a row (e.g. a
// concurrent first-time setup request inserted first) nothing is inserted and it
// returns (nil, nil); the caller re-fetches and reuses that existing enrollment.
// This keeps get-or-create idempotent under concurrent setup without ever
// violating the user_id unique constraint.
//
// [Ja] Create は未有効化の 2FA 設定 (登録用 secret) を挿入し、DB が採番した id と
// タイムスタンプを設定した状態で返します。挿入は ON CONFLICT (user_id) DO NOTHING の
// ため、ユーザーの行が既に存在するとき (例: 同時の初回設定リクエストが先に挿入したとき) は
// 何も挿入せず (nil, nil) を返します。呼び出し側はその既存の登録を取り直して再利用します。
// これにより、設定が同時に走っても user_id の unique 制約に一切違反せず get-or-create を
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

// Enable activates the user's not-yet-enabled 2FA setting: it marks the setting
// enabled, stamps enabled_at, and stores the generated recovery codes, all in one
// update guarded by enabled = false. It returns whether a row was actually enabled:
// false means no not-yet-enabled row matched (2FA was already enabled — e.g. a
// concurrent enable won the race — or the user never enrolled), so the caller's
// recovery codes were not stored and must not be shown. The guard makes enabling
// idempotent and stops a second concurrent enable from overwriting the stored
// recovery codes.
//
// [Ja] Enable はユーザーの未有効化の 2FA 設定を有効化します。設定を enabled にし、
// enabled_at を打刻し、生成したリカバリーコードを保存する処理を、enabled = false で
// ガードした 1 回の更新で行います。実際に行を有効化したかを返します。false は未有効化の行が
// 一致しなかった (2FA が既に有効 — 例えば同時の有効化が競合に勝った — か、ユーザーが登録して
// いない) ことを意味し、その場合呼び出し側のリカバリーコードは保存されておらず表示しては
// なりません。このガードにより有効化は冪等になり、2 つ目の同時有効化が保存済みリカバリー
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

// UpdateRecoveryCodes replaces the user's stored recovery codes with the given
// slice (and bumps updated_at). Sign-in with a recovery code uses this to write
// back the remaining codes after consuming one.
//
// [Ja] UpdateRecoveryCodes はユーザーの保存済みリカバリーコードを与えられたスライスで
// 置き換えます (updated_at も更新)。リカバリーコードでのサインインが、1 つ消費した後に
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

// Delete removes the user's 2FA setting, disabling two-factor authentication and
// discarding the secret and recovery codes with the row. Deleting when the user
// has no setting is not an error, so disabling is idempotent.
//
// [Ja] Delete はユーザーの 2FA 設定を削除し、2 段階認証を無効化して secret と
// リカバリーコードを行ごと破棄します。設定が無いときに削除してもエラーにならないため、
// 無効化は冪等です。
func (r *UserTwoFactorAuthRepository) Delete(ctx context.Context, userID model.UserID) error {
	return r.writer.DeleteUserTwoFactorAuthByUserID(ctx, int64(userID))
}

// toModel converts a query.UserTwoFactorAuth row into a model.UserTwoFactorAuth,
// casting the raw ids into the typed IDs and decoding the stored recovery codes
// at the repository boundary. It reports an error when the stored codes are not
// the JSON array the column is supposed to hold.
//
// [Ja] toModel は query.UserTwoFactorAuth を model.UserTwoFactorAuth に変換し、
// リポジトリの境界で生の id を型付き ID にキャストし、保存されたリカバリーコードを
// デコードします。保存された値が、その列が保持するはずの JSON 配列になっていない場合は
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

// encodeRecoveryCodes renders the recovery codes as the JSON array the column
// stores. SQLite has no array type, so a list-valued column is TEXT holding JSON
// and the repository boundary is where the slice becomes that text.
//
// [Ja] encodeRecoveryCodes はリカバリーコードを、列が保存する JSON 配列として表現
// します。SQLite に配列型は無いため、リストを値に取る列は JSON を保持する TEXT であり、
// スライスがそのテキストになる場所がリポジトリの境界です。
func encodeRecoveryCodes(recoveryCodes []string) (string, error) {
	// A nil slice encodes as "null", which the column's json_type check rejects.
	// An empty list is written as the empty array the column also defaults to.
	//
	// [Ja] nil のスライスは "null" にエンコードされ、列の json_type チェックに弾かれる。
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

// decodeRecoveryCodes parses the JSON array the column stores back into a slice.
//
// [Ja] decodeRecoveryCodes は列が保存する JSON 配列をスライスに解釈し直します。
func decodeRecoveryCodes(encoded string) ([]string, error) {
	var recoveryCodes []string
	if err := json.Unmarshal([]byte(encoded), &recoveryCodes); err != nil {
		return nil, fmt.Errorf("failed to decode the recovery codes: %w", err)
	}
	return recoveryCodes, nil
}
