package testutil

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/sqlitetime"
)

// DefaultBuilderTOTPSecret is the TOTP shared secret a UserTwoFactorAuthBuilder
// stores when none is set. It is a valid base32 string so a test that later
// exercises TOTP verification can reuse a known-good secret.
//
// [Ja] DefaultBuilderTOTPSecret は UserTwoFactorAuthBuilder が未設定時に保存する TOTP
// 共有シークレットです。有効な base32 文字列であり、後で TOTP 検証を行うテストが既知の
// 正しいシークレットとして再利用できるようにします。
const DefaultBuilderTOTPSecret = "JBSWY3DPEHPK3PXP"

// UserTwoFactorAuthBuilder builds a user_two_factor_auths row for tests via a
// fluent API. The owning user is required and has no default, since a 2FA setting
// always belongs to an existing user. The secret defaults to
// DefaultBuilderTOTPSecret, the setting defaults to disabled, and recovery codes
// default to empty, matching a freshly enrolled (not-yet-enabled) row. Setting
// Enabled stamps enabled_at on Build so an enabled setting looks like one the
// application produced.
//
// [Ja] UserTwoFactorAuthBuilder はテスト用の user_two_factor_auths 行を fluent API で
// 組み立てます。2FA 設定は常に既存ユーザーに属するため、所有ユーザーは必須で既定値は
// ありません。secret は DefaultBuilderTOTPSecret、設定は無効、リカバリーコードは空を
// 既定とし、登録直後 (未有効化) の行に一致します。Enabled を設定すると Build 時に
// enabled_at を打刻し、有効な設定がアプリケーションの生成したものと同じ見え方になります。
type UserTwoFactorAuthBuilder struct {
	t             *testing.T
	db            *database.DB
	userID        model.UserID
	secret        string
	enabled       bool
	recoveryCodes []string
}

// NewUserTwoFactorAuthBuilder creates a UserTwoFactorAuthBuilder with the default
// secret, disabled, and no recovery codes.
//
// [Ja] NewUserTwoFactorAuthBuilder は既定の secret を持ち、無効かつリカバリーコード
// 無しの UserTwoFactorAuthBuilder を生成します。
func NewUserTwoFactorAuthBuilder(t *testing.T, db *database.DB) *UserTwoFactorAuthBuilder {
	t.Helper()
	return &UserTwoFactorAuthBuilder{
		t:             t,
		db:            db,
		secret:        DefaultBuilderTOTPSecret,
		recoveryCodes: []string{},
	}
}

// WithUserID sets the owning user.
//
// [Ja] WithUserID は所有ユーザーを設定します。
func (b *UserTwoFactorAuthBuilder) WithUserID(userID model.UserID) *UserTwoFactorAuthBuilder {
	b.userID = userID
	return b
}

// WithSecret sets the TOTP shared secret, for tests that need a specific known
// secret.
//
// [Ja] WithSecret は TOTP 共有シークレットを設定します。特定の既知のシークレットが
// 必要なテストで使います。
func (b *UserTwoFactorAuthBuilder) WithSecret(secret string) *UserTwoFactorAuthBuilder {
	b.secret = secret
	return b
}

// WithEnabled marks the setting enabled, so Build produces an active 2FA setting
// (enabled_at is stamped on Build).
//
// [Ja] WithEnabled は設定を有効にし、Build が有効な 2FA 設定を生成するようにします
// (enabled_at は Build 時に打刻されます)。
func (b *UserTwoFactorAuthBuilder) WithEnabled(enabled bool) *UserTwoFactorAuthBuilder {
	b.enabled = enabled
	return b
}

// WithRecoveryCodes sets the recovery codes, for tests that need to seed a known
// set of one-time codes.
//
// [Ja] WithRecoveryCodes はリカバリーコードを設定します。既知の 1 回使い切りコードを
// 投入する必要があるテストで使います。
func (b *UserTwoFactorAuthBuilder) WithRecoveryCodes(recoveryCodes []string) *UserTwoFactorAuthBuilder {
	b.recoveryCodes = recoveryCodes
	return b
}

// Build inserts the 2FA setting and returns its database-assigned ID, failing
// the test on error. id and the timestamps are left to the database defaults,
// except enabled_at, which is set when the setting is enabled. It fails the test
// when no user has been set, since user_id is NOT NULL.
//
// [Ja] Build は 2FA 設定を挿入し、DB が採番した ID を返します。エラー時はテストを
// 失敗させます。id とタイムスタンプは DB の既定値に任せますが、enabled_at は設定が
// 有効なときに設定します。user_id は NOT NULL のため、ユーザーが未設定の場合はテストを
// 失敗させます。
func (b *UserTwoFactorAuthBuilder) Build() model.UserTwoFactorAuthID {
	b.t.Helper()

	if b.userID == 0 {
		b.t.Fatal("UserTwoFactorAuthBuilder にはユーザー ID が必要です (WithUserID で設定してください)")
	}

	var enabledAt *time.Time
	if b.enabled {
		now := time.Now()
		enabledAt = &now
	}

	// SQLite has no array type, so the column holds a JSON array; the builder
	// writes the same text the repository does.
	//
	// [Ja] SQLite に配列型は無く列は JSON 配列を保持するため、ビルダーはリポジトリと
	// 同じテキストを書く。
	recoveryCodes, err := json.Marshal(b.recoveryCodes)
	if err != nil {
		b.t.Fatalf("テスト用リカバリーコードのエンコードに失敗: %v", err)
	}

	var id int64
	err = b.db.Writer.QueryRowContext(context.Background(),
		`INSERT INTO user_two_factor_auths (user_id, secret, enabled, enabled_at, recovery_codes)
		 VALUES (?, ?, ?, ?, ?) RETURNING id`,
		int64(b.userID), b.secret, b.enabled, sqlitetime.Ptr(enabledAt), string(recoveryCodes),
	).Scan(&id)
	if err != nil {
		b.t.Fatalf("テスト用 2 段階認証設定の作成に失敗: %v", err)
	}

	return model.UserTwoFactorAuthID(id)
}
