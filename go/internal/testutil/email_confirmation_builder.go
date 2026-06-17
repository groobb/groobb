package testutil

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/groobb/groobb/go/internal/model"
)

// EmailConfirmationBuilder builds an email_confirmations row for tests via a
// fluent API, applying sensible defaults so a test only sets the fields it cares
// about.
//
// [Ja] EmailConfirmationBuilder はテスト用の email_confirmations 行を fluent API で
// 組み立てます。妥当な既定値を適用するため、テストは関心のあるフィールドだけを設定
// すれば済みます。
type EmailConfirmationBuilder struct {
	t                   *testing.T
	tx                  pgx.Tx
	email               string
	event               model.EmailConfirmationEvent
	code                string
	startedAt           *time.Time
	failedAttemptsCount int
}

// NewEmailConfirmationBuilder creates an EmailConfirmationBuilder. The default
// email embeds a random UUID so concurrent tests (t.Parallel) do not reuse the
// same address.
//
// [Ja] NewEmailConfirmationBuilder は EmailConfirmationBuilder を生成します。
// 既定の email にはランダムな UUID を埋め込み、並行テスト (t.Parallel) が同じ
// アドレスを使い回さないようにします。
func NewEmailConfirmationBuilder(t *testing.T, tx pgx.Tx) *EmailConfirmationBuilder {
	t.Helper()
	return &EmailConfirmationBuilder{
		t:     t,
		tx:    tx,
		email: fmt.Sprintf("confirm-%s@example.com", uuid.NewString()),
		event: model.EmailConfirmationEventSignUp,
		code:  "123456",
	}
}

// WithEmail sets the email being confirmed.
//
// [Ja] WithEmail は確認対象の email を設定します。
func (b *EmailConfirmationBuilder) WithEmail(email string) *EmailConfirmationBuilder {
	b.email = email
	return b
}

// WithEvent sets the confirmation event.
//
// [Ja] WithEvent は確認イベントを設定します。
func (b *EmailConfirmationBuilder) WithEvent(event model.EmailConfirmationEvent) *EmailConfirmationBuilder {
	b.event = event
	return b
}

// WithCode sets the confirmation code.
//
// [Ja] WithCode は確認コードを設定します。
func (b *EmailConfirmationBuilder) WithCode(code string) *EmailConfirmationBuilder {
	b.code = code
	return b
}

// WithStartedAt overrides started_at, the issue time that anchors the expiry
// window. A test sets this to a time more than the expiry window ago to build an
// expired confirmation; left unset, the row falls back to the database default
// (NOW()), i.e. a freshly issued, active confirmation.
//
// [Ja] WithStartedAt は有効期限ウィンドウの起点となる発行時刻 started_at を上書きします。
// テストは有効期限ウィンドウより前の時刻を渡して期限切れの確認を作ります。未設定なら
// DB の既定値 (NOW()) に委ね、発行直後のアクティブな確認になります。
func (b *EmailConfirmationBuilder) WithStartedAt(startedAt time.Time) *EmailConfirmationBuilder {
	b.startedAt = &startedAt
	return b
}

// WithFailedAttemptsCount overrides failed_attempts_count, the number of wrong
// code submissions. A test sets this to the limit to build a confirmation that
// has exhausted its attempts (no longer active); left unset, it defaults to 0,
// matching the database default for a freshly issued confirmation.
//
// [Ja] WithFailedAttemptsCount は誤ったコード送信回数 failed_attempts_count を上書き
// します。テストは上限値を渡して試行回数を使い切った確認 (もう active でない) を作ります。
// 未設定なら 0 を既定とし、発行直後の確認に対する DB の既定値と一致します。
func (b *EmailConfirmationBuilder) WithFailedAttemptsCount(count int) *EmailConfirmationBuilder {
	b.failedAttemptsCount = count
	return b
}

// Build inserts the confirmation and returns its database-assigned ID, failing
// the test on error. id and the timestamps are left to the database defaults,
// and succeeded_at starts NULL; started_at also defaults unless WithStartedAt
// overrode it (to build an expired confirmation). failed_attempts_count is
// always supplied, defaulting to 0 (the same as the database default) unless
// WithFailedAttemptsCount overrode it (to build an attempt-exhausted confirmation).
//
// [Ja] Build は確認を挿入し、DB が採番した ID を返します。エラー時はテストを失敗
// させます。id とタイムスタンプは DB の既定値に任せ、succeeded_at は NULL で始まり
// ます。started_at も WithStartedAt で上書きしない限り既定値に委ねます (上書きは
// 期限切れの確認を作るため)。failed_attempts_count は常に渡し、WithFailedAttemptsCount
// で上書きしない限り 0 (DB の既定値と同じ) を既定とします (上書きは試行回数を使い切った
// 確認を作るため)。
func (b *EmailConfirmationBuilder) Build() model.EmailConfirmationID {
	b.t.Helper()

	var id uuid.UUID
	var err error
	if b.startedAt != nil {
		err = b.tx.QueryRow(context.Background(),
			`INSERT INTO email_confirmations (email, event, code, started_at, failed_attempts_count) VALUES ($1, $2, $3, $4, $5) RETURNING id`,
			b.email, string(b.event), b.code, *b.startedAt, b.failedAttemptsCount,
		).Scan(&id)
	} else {
		err = b.tx.QueryRow(context.Background(),
			`INSERT INTO email_confirmations (email, event, code, failed_attempts_count) VALUES ($1, $2, $3, $4) RETURNING id`,
			b.email, string(b.event), b.code, b.failedAttemptsCount,
		).Scan(&id)
	}
	if err != nil {
		b.t.Fatalf("テスト用メール確認の作成に失敗: %v", err)
	}

	return model.EmailConfirmationID(id)
}
