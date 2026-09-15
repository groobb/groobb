package testutil

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/sqlitetime"
)

// EmailConfirmationBuilderはテスト用のemail_confirmations行をfluent APIで
// 組み立てます。妥当な既定値を適用するため、テストは関心のあるフィールドだけを設定
// すれば済みます。
type EmailConfirmationBuilder struct {
	t                   *testing.T
	db                  *database.DB
	userID              *int64
	email               string
	event               model.EmailConfirmationEvent
	code                string
	startedAt           *time.Time
	failedAttemptsCount int
}

// NewEmailConfirmationBuilderはEmailConfirmationBuilderを生成します。
// 既定のemailはそのテストの次の連番を持つため、複数の確認を作るテストがそれらの間で
// 1つのアドレスを使い回すことはありません。
func NewEmailConfirmationBuilder(t *testing.T, db *database.DB) *EmailConfirmationBuilder {
	t.Helper()
	return &EmailConfirmationBuilder{
		t:     t,
		db:    db,
		email: fmt.Sprintf("confirm-%d@example.com", nextSequence(db)),
		event: model.EmailConfirmationEventSignUp,
		code:  "123456",
	}
}

// WithUserIDは確認をユーザーに紐付けます (メール変更の確認がそうであるように)。
// 未設定ならuser_idはNULLのまま (ユーザーが存在する前に発行されるサインアップの確認の
// 既定) です。
func (b *EmailConfirmationBuilder) WithUserID(userID model.UserID) *EmailConfirmationBuilder {
	id := int64(userID)
	b.userID = &id
	return b
}

// WithEmailは確認対象のemailを設定します。
func (b *EmailConfirmationBuilder) WithEmail(email string) *EmailConfirmationBuilder {
	b.email = email
	return b
}

// WithEventは確認イベントを設定します。
func (b *EmailConfirmationBuilder) WithEvent(event model.EmailConfirmationEvent) *EmailConfirmationBuilder {
	b.event = event
	return b
}

// WithCodeは確認コードを設定します。
func (b *EmailConfirmationBuilder) WithCode(code string) *EmailConfirmationBuilder {
	b.code = code
	return b
}

// WithStartedAtは有効期限ウィンドウの起点となる発行時刻started_atを上書きします。
// テストは有効期限ウィンドウより前の時刻を渡して期限切れの確認を作ります。未設定なら
// DBの既定値 (NOW()) に委ね、発行直後のアクティブな確認になります。
func (b *EmailConfirmationBuilder) WithStartedAt(startedAt time.Time) *EmailConfirmationBuilder {
	b.startedAt = &startedAt
	return b
}

// WithFailedAttemptsCountは誤ったコード送信回数failed_attempts_countを上書き
// します。テストは上限値を渡して試行回数を使い切った確認 (もうactiveでない) を作ります。
// 未設定なら0を既定とし、発行直後の確認に対するDBの既定値と一致します。
func (b *EmailConfirmationBuilder) WithFailedAttemptsCount(count int) *EmailConfirmationBuilder {
	b.failedAttemptsCount = count
	return b
}

// Buildは確認を挿入し、DBが採番したIDを返します。エラー時はテストを失敗
// させます。idとタイムスタンプはDBの既定値に任せ、succeeded_atはNULLで始まり
// ます。started_atもWithStartedAtで上書きしない限り既定値に委ねます (上書きは
// 期限切れの確認を作るため)。failed_attempts_countは常に渡し、WithFailedAttemptsCount
// で上書きしない限り0 (DBの既定値と同じ) を既定とします (上書きは試行回数を使い切った
// 確認を作るため)。user_idも常に渡し、WithUserIDで設定しない限りNULLを既定とします
// (設定はユーザーに紐付いたメール変更の確認を作るため)。
func (b *EmailConfirmationBuilder) Build() model.EmailConfirmationID {
	b.t.Helper()

	var id int64
	var err error
	if b.startedAt != nil {
		err = b.db.Writer.QueryRowContext(context.Background(),
			`INSERT INTO email_confirmations (user_id, email, event, code, started_at, failed_attempts_count) VALUES (?, ?, ?, ?, ?, ?) RETURNING id`,
			b.userID, b.email, string(b.event), b.code, sqlitetime.Time(*b.startedAt), b.failedAttemptsCount,
		).Scan(&id)
	} else {
		err = b.db.Writer.QueryRowContext(context.Background(),
			`INSERT INTO email_confirmations (user_id, email, event, code, failed_attempts_count) VALUES (?, ?, ?, ?, ?) RETURNING id`,
			b.userID, b.email, string(b.event), b.code, b.failedAttemptsCount,
		).Scan(&id)
	}
	if err != nil {
		b.t.Fatalf("テスト用メール確認の作成に失敗: %v", err)
	}

	return model.EmailConfirmationID(id)
}
