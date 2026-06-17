package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/query"
)

// EmailConfirmationRepository reads and writes email_confirmations through
// sqlc-generated queries.
//
// [Ja] EmailConfirmationRepository は sqlc 生成のクエリ経由で email_confirmations を
// 読み書きします。
type EmailConfirmationRepository struct {
	q *query.Queries
}

// NewEmailConfirmationRepository creates an EmailConfirmationRepository backed by
// the given queries.
//
// [Ja] NewEmailConfirmationRepository は与えられた queries を使う
// EmailConfirmationRepository を生成します。
func NewEmailConfirmationRepository(q *query.Queries) *EmailConfirmationRepository {
	return &EmailConfirmationRepository{q: q}
}

// WithTx returns a new EmailConfirmationRepository whose queries run inside tx,
// so a UseCase can enlist this repository in its transaction. The receiver is
// left unchanged.
//
// [Ja] WithTx は queries を tx 内で実行する新しい EmailConfirmationRepository を返し、
// UseCase が本リポジトリを自身のトランザクションに参加させられるようにします。
// レシーバ自身は変更しません。
func (r *EmailConfirmationRepository) WithTx(tx pgx.Tx) *EmailConfirmationRepository {
	return &EmailConfirmationRepository{q: r.q.WithTx(tx)}
}

// CreateEmailConfirmationInput holds the attributes needed to create a
// confirmation. id, started_at, and the timestamps are assigned by the database,
// and succeeded_at starts NULL.
//
// [Ja] CreateEmailConfirmationInput は確認の作成に必要な属性を保持します。id /
// started_at / タイムスタンプは DB 側で採番され、succeeded_at は NULL で始まります。
type CreateEmailConfirmationInput struct {
	Email string
	Event model.EmailConfirmationEvent
	Code  string
}

// Create inserts a confirmation and returns it with the database-assigned id and
// timestamps populated.
//
// [Ja] Create は確認を挿入し、DB が採番した id とタイムスタンプを設定した状態で
// 返します。
func (r *EmailConfirmationRepository) Create(ctx context.Context, input CreateEmailConfirmationInput) (*model.EmailConfirmation, error) {
	row, err := r.q.CreateEmailConfirmation(ctx, query.CreateEmailConfirmationParams{
		Email: input.Email,
		Event: string(input.Event),
		Code:  input.Code,
	})
	if err != nil {
		return nil, err
	}
	return r.toModel(row), nil
}

// FindActiveByID returns the still-usable confirmation with the given id, or
// (nil, nil) when none qualifies. "Active" is decided in SQL: not yet succeeded
// and still inside the 15-minute window measured from started_at (the issue
// time), which is the Korylus-wide expiry convention. The lookup is keyed by id
// alone — the primary key carried in the sign-up handoff cookie — so the
// primary-key index already serves it and no secondary index is needed. An
// already-succeeded, expired, or unknown id all surface as (nil, nil); a non-nil
// error is reserved for a genuine query failure.
//
// [Ja] FindActiveByID は指定 id のまだ使える確認を返し、該当が無ければ (nil, nil) を
// 返します。"active" の判定は SQL 側で行います。未確認かつ、started_at (発行時刻) から
// 測った 15 分のウィンドウ内であること (Korylus 共通の有効期限の慣行) です。ルックアップ
// は id のみ — サインアップの受け渡し Cookie が運ぶ主キー — をキーにするため、主キー
// インデックスがそのまま使え、二次インデックスは不要です。確認済み・期限切れ・未知の id
// はいずれも (nil, nil) として表れ、非 nil のエラーは本物のクエリ失敗のためにのみ用います。
func (r *EmailConfirmationRepository) FindActiveByID(ctx context.Context, id model.EmailConfirmationID) (*model.EmailConfirmation, error) {
	row, err := r.q.GetActiveEmailConfirmationByID(ctx, uuid.UUID(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return r.toModel(row), nil
}

// FindSucceededByID returns the already-verified confirmation with the given id,
// or (nil, nil) when none qualifies. "Succeeded" is decided in SQL (succeeded_at
// IS NOT NULL): only a confirmation whose code was already accepted matches, so
// account creation reads the verified email from a confirmation the user has
// proven control of. The lookup is keyed by id alone (the primary key carried in
// the sign-up handoff cookie), so the primary-key index already serves it. There
// is no extra time window here: the handoff cookie's own 15-minute lifetime
// bounds how long the verified confirmation stays usable. An unknown or
// not-yet-succeeded id surfaces as (nil, nil); a non-nil error is reserved for a
// genuine query failure.
//
// [Ja] FindSucceededByID は指定 id の検証済み確認を返し、該当が無ければ (nil, nil) を
// 返します。"succeeded" の判定は SQL 側 (succeeded_at IS NOT NULL) で行い、コードが
// 既に受理された確認だけがマッチするため、アカウント作成はユーザーが管理権を証明済みの
// 確認から検証済み email を読めます。ルックアップは id のみ (サインアップの受け渡し
// Cookie が運ぶ主キー) をキーにするため、主キーインデックスがそのまま使えます。ここでは
// 追加の時間ウィンドウは設けません。受け渡し Cookie 自身の 15 分の寿命が、検証済み確認が
// 使える期間を区切ります。未知 / 未成功の id は (nil, nil) として表れ、非 nil のエラーは
// 本物のクエリ失敗のためにのみ用います。
func (r *EmailConfirmationRepository) FindSucceededByID(ctx context.Context, id model.EmailConfirmationID) (*model.EmailConfirmation, error) {
	row, err := r.q.GetSucceededEmailConfirmationByID(ctx, uuid.UUID(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return r.toModel(row), nil
}

// Succeed stamps the confirmation's succeeded_at (and updated_at) to mark its
// code as accepted, so it can no longer be matched by FindActiveByID and the
// flow can advance to account creation.
//
// [Ja] Succeed は確認の succeeded_at (と updated_at) を打刻してコードが受理されたことを
// 記録します。これにより FindActiveByID で再びマッチしなくなり、フローはアカウント作成へ
// 進めます。
func (r *EmailConfirmationRepository) Succeed(ctx context.Context, id model.EmailConfirmationID) error {
	return r.q.UpdateEmailConfirmationSucceededAt(ctx, uuid.UUID(id))
}

// IncrementFailedAttempts bumps the confirmation's failed_attempts_count by one
// (and updates updated_at) after a wrong code is submitted. The increment is a
// single atomic UPDATE (count = count + 1), so each wrong attempt is counted
// reliably; once the count reaches the limit, FindActiveByID stops returning the
// row and the user must request a new code from sign-up.
//
// [Ja] IncrementFailedAttempts は誤ったコードが送信された後、確認の
// failed_attempts_count を 1 増やします (updated_at も更新)。インクリメントは単一の
// アトミックな UPDATE (count = count + 1) のため、誤った試行が確実に数えられます。
// 上限に達すると FindActiveByID は当該行を返さなくなり、ユーザーはサインアップから
// 新しいコードを再申請する必要があります。
func (r *EmailConfirmationRepository) IncrementFailedAttempts(ctx context.Context, id model.EmailConfirmationID) error {
	return r.q.IncrementEmailConfirmationFailedAttempts(ctx, uuid.UUID(id))
}

// toModel converts a query.EmailConfirmation row into a model.EmailConfirmation,
// casting the raw uuid, event string, and int32 count into their typed forms at
// the repository boundary.
//
// [Ja] toModel は query.EmailConfirmation を model.EmailConfirmation に変換し、
// リポジトリの境界で生の uuid・event 文字列・int32 のカウントを型付きの形に
// キャストします。
func (r *EmailConfirmationRepository) toModel(row query.EmailConfirmation) *model.EmailConfirmation {
	return &model.EmailConfirmation{
		ID:                  model.EmailConfirmationID(row.ID),
		Email:               row.Email,
		Event:               model.EmailConfirmationEvent(row.Event),
		Code:                row.Code,
		StartedAt:           row.StartedAt,
		SucceededAt:         row.SucceededAt,
		FailedAttemptsCount: int(row.FailedAttemptsCount),
		CreatedAt:           row.CreatedAt,
		UpdatedAt:           row.UpdatedAt,
	}
}
