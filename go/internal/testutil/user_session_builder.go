package testutil

import (
	"context"
	"fmt"
	"testing"

	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/model"
)

// UserSessionBuilder builds a user_sessions row for tests via a fluent API,
// applying sensible defaults so a test only sets the fields it cares about. The
// owning user is required and has no default, since a session always belongs to
// an existing user.
//
// [Ja] UserSessionBuilder はテスト用の user_sessions 行を fluent API で組み立てます。
// 妥当な既定値を適用するため、テストは関心のあるフィールドだけを設定すれば済みます。
// セッションは常に既存ユーザーに属するため、所有ユーザーは必須で既定値はありません。
type UserSessionBuilder struct {
	t         *testing.T
	db        *database.DB
	userID    model.UserID
	token     string
	ipAddress string
	userAgent string
}

// NewUserSessionBuilder creates a UserSessionBuilder. The default token carries
// the test's next sequence number, so a test that builds several sessions does
// not have to name each token to keep them apart on the user_sessions.token
// UNIQUE constraint.
//
// [Ja] NewUserSessionBuilder は UserSessionBuilder を生成します。既定の token はその
// テストの次の連番を持つため、複数のセッションを作るテストが user_sessions.token の
// UNIQUE 制約で互いを区別するために token を一つずつ決める必要はありません。
func NewUserSessionBuilder(t *testing.T, db *database.DB) *UserSessionBuilder {
	t.Helper()
	return &UserSessionBuilder{
		t:         t,
		db:        db,
		token:     fmt.Sprintf("test-token-%d", nextSequence(db)),
		ipAddress: "127.0.0.1",
		userAgent: "test-user-agent",
	}
}

// WithUserID sets the owning user.
//
// [Ja] WithUserID は所有ユーザーを設定します。
func (b *UserSessionBuilder) WithUserID(userID model.UserID) *UserSessionBuilder {
	b.userID = userID
	return b
}

// WithToken sets the session token.
//
// [Ja] WithToken はセッショントークンを設定します。
func (b *UserSessionBuilder) WithToken(token string) *UserSessionBuilder {
	b.token = token
	return b
}

// WithIPAddress sets the IP address.
//
// [Ja] WithIPAddress は IP アドレスを設定します。
func (b *UserSessionBuilder) WithIPAddress(ipAddress string) *UserSessionBuilder {
	b.ipAddress = ipAddress
	return b
}

// WithUserAgent sets the user agent.
//
// [Ja] WithUserAgent は User-Agent を設定します。
func (b *UserSessionBuilder) WithUserAgent(userAgent string) *UserSessionBuilder {
	b.userAgent = userAgent
	return b
}

// Build inserts the session and returns its database-assigned ID, failing the
// test on error. id and timestamps are left to the database defaults. It fails
// the test when no user has been set, since user_id is NOT NULL.
//
// [Ja] Build はセッションを挿入し、DB が採番した ID を返します。エラー時はテストを
// 失敗させます。id とタイムスタンプは DB の既定値に任せます。user_id は NOT NULL の
// ため、ユーザーが未設定の場合はテストを失敗させます。
func (b *UserSessionBuilder) Build() model.UserSessionID {
	b.t.Helper()

	if b.userID == 0 {
		b.t.Fatal("UserSessionBuilder にはユーザー ID が必要です (WithUserID で設定してください)")
	}

	var id int64
	err := b.db.Writer.QueryRowContext(context.Background(),
		`INSERT INTO user_sessions (user_id, token, ip_address, user_agent) VALUES (?, ?, ?, ?) RETURNING id`,
		int64(b.userID), b.token, b.ipAddress, b.userAgent,
	).Scan(&id)
	if err != nil {
		b.t.Fatalf("テスト用ユーザーセッションの作成に失敗: %v", err)
	}

	return model.UserSessionID(id)
}
