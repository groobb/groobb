package testutil

import (
	"context"
	"fmt"
	"testing"

	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/model"
)

// UserSessionBuilderはテスト用のuser_sessions行をfluent APIで組み立てます。
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

// NewUserSessionBuilderはUserSessionBuilderを生成します。既定のtokenはその
// テストの次の連番を持つため、複数のセッションを作るテストがuser_sessions.tokenの
// UNIQUE制約で互いを区別するためにtokenを一つずつ決める必要はありません。
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

// WithUserIDは所有ユーザーを設定します。
func (b *UserSessionBuilder) WithUserID(userID model.UserID) *UserSessionBuilder {
	b.userID = userID
	return b
}

// WithTokenはセッショントークンを設定します。
func (b *UserSessionBuilder) WithToken(token string) *UserSessionBuilder {
	b.token = token
	return b
}

// WithIPAddressはIPアドレスを設定します。
func (b *UserSessionBuilder) WithIPAddress(ipAddress string) *UserSessionBuilder {
	b.ipAddress = ipAddress
	return b
}

// WithUserAgentはUser-Agentを設定します。
func (b *UserSessionBuilder) WithUserAgent(userAgent string) *UserSessionBuilder {
	b.userAgent = userAgent
	return b
}

// Buildはセッションを挿入し、DBが採番したIDを返します。エラー時はテストを
// 失敗させます。idとタイムスタンプはDBの既定値に任せます。user_idはNOT NULLの
// ため、ユーザーが未設定の場合はテストを失敗させます。
func (b *UserSessionBuilder) Build() model.UserSessionID {
	b.t.Helper()

	if b.userID == 0 {
		b.t.Fatal("UserSessionBuilderにはユーザーIDが必要です (WithUserIDで設定してください)")
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
