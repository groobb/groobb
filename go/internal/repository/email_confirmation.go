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

// EmailConfirmationRepositoryはsqlc生成のクエリ経由でemail_confirmationsを
// 読み書きします。
type EmailConfirmationRepository struct {
	reader *query.Queries
	writer *query.Queries
}

// NewEmailConfirmationRepositoryは、データベースの読み取り用プールで読み、書き込み用プールで書く
// EmailConfirmationRepositoryを生成します。
func NewEmailConfirmationRepository(db *database.DB) *EmailConfirmationRepository {
	return &EmailConfirmationRepository{reader: query.New(db.Reader), writer: query.New(db.Writer)}
}

// WithTxはqueriesをtx内で実行する新しいEmailConfirmationRepositoryを返し、
// UseCaseが本リポジトリを自身のトランザクションに参加させられるようにします。
// レシーバ自身は変更しません。
func (r *EmailConfirmationRepository) WithTx(tx *sql.Tx) *EmailConfirmationRepository {
	q := r.writer.WithTx(tx)
	return &EmailConfirmationRepository{reader: q, writer: q}
}

// CreateEmailConfirmationInputは確認の作成に必要な属性を保持します。id /
// started_at / タイムスタンプはDB側で採番され、succeeded_atはNULLで始まります。
type CreateEmailConfirmationInput struct {
	Email string
	Event model.EmailConfirmationEvent
	Code  string
}

// Createは確認を挿入し、DBが採番したidとタイムスタンプを設定した状態で
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

// FindActiveByIDは指定idのまだ使える確認を返し、該当が無ければ (nil, nil) を
// 返します。"active" の判定はSQL側で行います。未確認かつ、started_at (発行時刻) から
// 測った15分のウィンドウ内であること (Korylus共通の有効期限の慣行) です。ルックアップ
// はidのみ — サインアップの受け渡しCookieが運ぶ主キー — をキーにするため、主キー
// インデックスがそのまま使え、二次インデックスは不要です。確認済み・期限切れ・未知のid
// はいずれも (nil, nil) として表れ、非nilのエラーは本物のクエリ失敗のためにのみ用います。
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

// FindSucceededByIDは指定idの検証済み確認を返し、該当が無ければ (nil, nil) を
// 返します。"succeeded" の判定はSQL側 (succeeded_at IS NOT NULL) で行い、コードが
// 既に受理された確認だけがマッチするため、アカウント作成はユーザーが管理権を証明済みの
// 確認から検証済みemailを読めます。ルックアップはidのみ (サインアップの受け渡し
// Cookieが運ぶ主キー) をキーにするため、主キーインデックスがそのまま使えます。ここでは
// 追加の時間ウィンドウは設けません。受け渡しCookie自身の15分の寿命が、検証済み確認が
// 使える期間を区切ります。未知 / 未成功のidは (nil, nil) として表れ、非nilのエラーは
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

// Succeedは確認のsucceeded_at (とupdated_at) を打刻してコードが受理されたことを
// 記録します。これによりFindActiveByIDで再びマッチしなくなり、フローはアカウント作成へ
// 進めます。
func (r *EmailConfirmationRepository) Succeed(ctx context.Context, id model.EmailConfirmationID) error {
	return r.writer.UpdateEmailConfirmationSucceededAt(ctx, int64(id))
}

// IncrementFailedAttemptsは誤ったコードが送信された後、確認の
// failed_attempts_countを1増やします (updated_atも更新)。インクリメントは単一の
// アトミックなUPDATE (count = count + 1) のため、誤った試行が確実に数えられます。
// 上限に達するとFindActiveByIDは当該行を返さなくなり、ユーザーはサインアップから
// 新しいコードを再申請する必要があります。
func (r *EmailConfirmationRepository) IncrementFailedAttempts(ctx context.Context, id model.EmailConfirmationID) error {
	return r.writer.IncrementEmailConfirmationFailedAttempts(ctx, int64(id))
}

// CreateEmailChangeInputはメール変更の確認の作成に必要な属性を保持します。
// サインアップの確認と違い申請したユーザーのidを持ち、Emailはユーザーが切り替えたい
// 新しいアドレスです。eventはクエリ側でemail_changeに固定され、id / started_at /
// タイムスタンプはDB側で採番され、succeeded_atはNULLで始まります。
type CreateEmailChangeInput struct {
	UserID model.UserID
	Email  string
	Code   string
}

// CreateEmailChangeは指定ユーザーと新しいアドレスに対するメール変更の確認を挿入し、
// DBが採番したidとタイムスタンプを設定した状態で返します。
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

// FindActiveEmailChangeByUserIDは指定ユーザーのまだ使えるメール変更の確認を返し、
// 該当が無ければ (nil, nil) を返します。"active" の判定はSQL側で行います。eventが
// email_changeで、未確認、started_atから測った15分のウィンドウ内、かつ5回の試行上限
// 未満であること — サインアップのルックアップと同じ有効期限・総当たりの規則です。主キー
// ではなくuser_idをキーにするのは、メール変更の確認ステップがhandoff Cookieではなく
// セッションのユーザーから保留中の確認を特定するためです。DeleteUnusedEmailChangesByUserID
// がユーザーごとにactiveを高々1件に保ち、万一その不変条件が崩れてもORDER BY
// started_at DESCが最新を返します。該当なし・確認済み・期限切れ・試行超過の確認はいずれも
// (nil, nil) として表れ、非nilのエラーは本物のクエリ失敗のためにのみ用います。
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

// DeleteUnusedEmailChangesByUserIDは指定ユーザーの未確認のメール変更の確認を削除し、
// 新しい申請の発行がまっさらな状態から始まり、ユーザーごとに保留中の確認が高々1件に
// なるようにします。確認済みのものは変更が成立した記録として残します。ユーザーに保留中の
// メール変更の確認が無ければ何もしません。
func (r *EmailConfirmationRepository) DeleteUnusedEmailChangesByUserID(ctx context.Context, userID model.UserID) error {
	id := int64(userID)
	return r.writer.DeleteUnusedEmailChangeConfirmationsByUserID(ctx, &id)
}

// toModelはquery.EmailConfirmationをmodel.EmailConfirmationに変換し、
// リポジトリの境界で生のid・event文字列・整数のカウントを型付きの形に
// キャストします。UserIDはサインアップの確認ではnilのまま (列がNULL)、メール変更の
// 確認では型付きの *UserIDになります。
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
