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

// EmailConfirmationRepository reads and writes email_confirmations through
// sqlc-generated queries.
//
// [Ja] EmailConfirmationRepository は sqlc 生成のクエリ経由で email_confirmations を
// 読み書きします。
type EmailConfirmationRepository struct {
	reader *query.Queries
	writer *query.Queries
}

// NewEmailConfirmationRepository creates a EmailConfirmationRepository that reads through the database's read pool
// and writes through its write pool.
//
// [Ja] NewEmailConfirmationRepository は、データベースの読み取り用プールで読み、書き込み用プールで書く
// EmailConfirmationRepository を生成します。
func NewEmailConfirmationRepository(db *database.DB) *EmailConfirmationRepository {
	return &EmailConfirmationRepository{reader: query.New(db.Reader), writer: query.New(db.Writer)}
}

// WithTx returns a new EmailConfirmationRepository whose queries run inside tx,
// so a UseCase can enlist this repository in its transaction. The receiver is
// left unchanged.
//
// [Ja] WithTx は queries を tx 内で実行する新しい EmailConfirmationRepository を返し、
// UseCase が本リポジトリを自身のトランザクションに参加させられるようにします。
// レシーバ自身は変更しません。
func (r *EmailConfirmationRepository) WithTx(tx *sql.Tx) *EmailConfirmationRepository {
	q := r.writer.WithTx(tx)
	return &EmailConfirmationRepository{reader: q, writer: q}
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
	row, err := r.writer.CreateEmailConfirmation(ctx, query.CreateEmailConfirmationParams{
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
	row, err := r.reader.GetActiveEmailConfirmationByID(ctx, int64(id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
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
	row, err := r.reader.GetSucceededEmailConfirmationByID(ctx, int64(id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
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
	return r.writer.UpdateEmailConfirmationSucceededAt(ctx, int64(id))
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
	return r.writer.IncrementEmailConfirmationFailedAttempts(ctx, int64(id))
}

// CreateEmailChangeInput holds the attributes needed to create an email-change
// confirmation. Unlike a sign-up confirmation it carries the requesting user's
// id, and Email is the new address the user wants to switch to. The event is
// fixed to email_change by the query; id, started_at, and the timestamps are
// assigned by the database, and succeeded_at starts NULL.
//
// [Ja] CreateEmailChangeInput はメール変更の確認の作成に必要な属性を保持します。
// サインアップの確認と違い申請したユーザーの id を持ち、Email はユーザーが切り替えたい
// 新しいアドレスです。event はクエリ側で email_change に固定され、id / started_at /
// タイムスタンプは DB 側で採番され、succeeded_at は NULL で始まります。
type CreateEmailChangeInput struct {
	UserID model.UserID
	Email  string
	Code   string
}

// CreateEmailChange inserts an email-change confirmation for the given user and
// new address, returning it with the database-assigned id and timestamps
// populated.
//
// [Ja] CreateEmailChange は指定ユーザーと新しいアドレスに対するメール変更の確認を挿入し、
// DB が採番した id とタイムスタンプを設定した状態で返します。
func (r *EmailConfirmationRepository) CreateEmailChange(ctx context.Context, input CreateEmailChangeInput) (*model.EmailConfirmation, error) {
	userID := int64(input.UserID)
	row, err := r.writer.CreateEmailChangeConfirmation(ctx, query.CreateEmailChangeConfirmationParams{
		UserID: &userID,
		Email:  input.Email,
		Code:   input.Code,
	})
	if err != nil {
		return nil, err
	}
	return r.toModel(row), nil
}

// FindActiveEmailChangeByUserID returns the user's still-usable email-change
// confirmation, or (nil, nil) when none qualifies. "Active" is decided in SQL:
// the event is email_change, not yet succeeded, still inside the 15-minute window
// measured from started_at, and under the 5-attempt limit — the same expiry and
// brute-force rules the sign-up lookup uses. It is keyed by user_id rather than
// the primary key because the email-change confirm step identifies the pending
// confirmation from the session's user, not from a handoff cookie;
// DeleteUnusedEmailChangesByUserID keeps at most one active per user, and ORDER
// BY started_at DESC returns the newest should that invariant ever fail to hold.
// A missing, already-succeeded, expired, or attempt-exhausted confirmation all
// surface as (nil, nil); a non-nil error is reserved for a genuine query failure.
//
// [Ja] FindActiveEmailChangeByUserID は指定ユーザーのまだ使えるメール変更の確認を返し、
// 該当が無ければ (nil, nil) を返します。"active" の判定は SQL 側で行います。event が
// email_change で、未確認、started_at から測った 15 分のウィンドウ内、かつ 5 回の試行上限
// 未満であること — サインアップのルックアップと同じ有効期限・総当たりの規則です。主キー
// ではなく user_id をキーにするのは、メール変更の確認ステップが handoff Cookie ではなく
// セッションのユーザーから保留中の確認を特定するためです。DeleteUnusedEmailChangesByUserID
// がユーザーごとに active を高々 1 件に保ち、万一その不変条件が崩れても ORDER BY
// started_at DESC が最新を返します。該当なし・確認済み・期限切れ・試行超過の確認はいずれも
// (nil, nil) として表れ、非 nil のエラーは本物のクエリ失敗のためにのみ用います。
func (r *EmailConfirmationRepository) FindActiveEmailChangeByUserID(ctx context.Context, userID model.UserID) (*model.EmailConfirmation, error) {
	id := int64(userID)
	row, err := r.reader.GetActiveEmailChangeConfirmationByUserID(ctx, &id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return r.toModel(row), nil
}

// DeleteUnusedEmailChangesByUserID deletes the user's not-yet-succeeded
// email-change confirmations, so issuing a new request starts from a clean slate
// and at most one pending confirmation exists per user. Already-succeeded
// confirmations are left untouched as a record that a change completed. It is a
// no-op when the user has no pending email-change confirmation.
//
// [Ja] DeleteUnusedEmailChangesByUserID は指定ユーザーの未確認のメール変更の確認を削除し、
// 新しい申請の発行がまっさらな状態から始まり、ユーザーごとに保留中の確認が高々 1 件に
// なるようにします。確認済みのものは変更が成立した記録として残します。ユーザーに保留中の
// メール変更の確認が無ければ何もしません。
func (r *EmailConfirmationRepository) DeleteUnusedEmailChangesByUserID(ctx context.Context, userID model.UserID) error {
	id := int64(userID)
	return r.writer.DeleteUnusedEmailChangeConfirmationsByUserID(ctx, &id)
}

// toModel converts a query.EmailConfirmation row into a model.EmailConfirmation,
// casting the raw id, event string, and integer count into their typed forms at
// the repository boundary. UserID stays nil for sign-up confirmations (the
// column is NULL) and becomes a typed *UserID for email-change confirmations.
//
// [Ja] toModel は query.EmailConfirmation を model.EmailConfirmation に変換し、
// リポジトリの境界で生の id・event 文字列・整数のカウントを型付きの形に
// キャストします。UserID はサインアップの確認では nil のまま (列が NULL)、メール変更の
// 確認では型付きの *UserID になります。
func (r *EmailConfirmationRepository) toModel(row query.EmailConfirmation) *model.EmailConfirmation {
	var userID *model.UserID
	if row.UserID != nil {
		id := model.UserID(*row.UserID)
		userID = &id
	}
	return &model.EmailConfirmation{
		ID:                  model.EmailConfirmationID(row.ID),
		UserID:              userID,
		Email:               row.Email,
		Event:               model.EmailConfirmationEvent(row.Event),
		Code:                row.Code,
		StartedAt:           time.Time(row.StartedAt),
		SucceededAt:         sqlitetime.TimePtr(row.SucceededAt),
		FailedAttemptsCount: int(row.FailedAttemptsCount),
		CreatedAt:           time.Time(row.CreatedAt),
		UpdatedAt:           time.Time(row.UpdatedAt),
	}
}
