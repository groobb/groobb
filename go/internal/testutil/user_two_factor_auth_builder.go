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

// DefaultBuilderTOTPSecretはUserTwoFactorAuthBuilderが未設定時に保存するTOTP
// 共有シークレットです。有効なbase32文字列であり、後でTOTP検証を行うテストが既知の
// 正しいシークレットとして再利用できるようにします。
const DefaultBuilderTOTPSecret = "JBSWY3DPEHPK3PXP"

// UserTwoFactorAuthBuilderはテスト用のuser_two_factor_auths行をfluent APIで
// 組み立てます。2FA設定は常に既存ユーザーに属するため、所有ユーザーは必須で既定値は
// ありません。secretはDefaultBuilderTOTPSecret、設定は無効、リカバリーコードは空を
// 既定とし、登録直後 (未有効化) の行に一致します。Enabledを設定するとBuild時に
// enabled_atを打刻し、有効な設定がアプリケーションの生成したものと同じ見え方になります。
type UserTwoFactorAuthBuilder struct {
	t             *testing.T
	db            *database.DB
	userID        model.UserID
	secret        string
	enabled       bool
	recoveryCodes []string
}

// NewUserTwoFactorAuthBuilderは既定のsecretを持ち、無効かつリカバリーコード
// 無しのUserTwoFactorAuthBuilderを生成します。
func NewUserTwoFactorAuthBuilder(t *testing.T, db *database.DB) *UserTwoFactorAuthBuilder {
	t.Helper()
	return &UserTwoFactorAuthBuilder{
		t:             t,
		db:            db,
		secret:        DefaultBuilderTOTPSecret,
		recoveryCodes: []string{},
	}
}

// WithUserIDは所有ユーザーを設定します。
func (b *UserTwoFactorAuthBuilder) WithUserID(userID model.UserID) *UserTwoFactorAuthBuilder {
	b.userID = userID
	return b
}

// WithSecretはTOTP共有シークレットを設定します。特定の既知のシークレットが
// 必要なテストで使います。
func (b *UserTwoFactorAuthBuilder) WithSecret(secret string) *UserTwoFactorAuthBuilder {
	b.secret = secret
	return b
}

// WithEnabledは設定を有効にし、Buildが有効な2FA設定を生成するようにします
// (enabled_atはBuild時に打刻されます)。
func (b *UserTwoFactorAuthBuilder) WithEnabled(enabled bool) *UserTwoFactorAuthBuilder {
	b.enabled = enabled
	return b
}

// WithRecoveryCodesはリカバリーコードを設定します。既知の1回使い切りコードを
// 投入する必要があるテストで使います。
func (b *UserTwoFactorAuthBuilder) WithRecoveryCodes(recoveryCodes []string) *UserTwoFactorAuthBuilder {
	b.recoveryCodes = recoveryCodes
	return b
}

// Buildは2FA設定を挿入し、DBが採番したIDを返します。エラー時はテストを
// 失敗させます。idとタイムスタンプはDBの既定値に任せますが、enabled_atは設定が
// 有効なときに設定します。user_idはNOT NULLのため、ユーザーが未設定の場合はテストを
// 失敗させます。
func (b *UserTwoFactorAuthBuilder) Build() model.UserTwoFactorAuthID {
	b.t.Helper()

	if b.userID == 0 {
		b.t.Fatal("UserTwoFactorAuthBuilderにはユーザーIDが必要です (WithUserIDで設定してください)")
	}

	var enabledAt *time.Time
	if b.enabled {
		now := time.Now()
		enabledAt = &now
	}

	// SQLiteに配列型は無く列はJSON配列を保持するため、ビルダーはリポジトリと
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
		b.t.Fatalf("テスト用2段階認証設定の作成に失敗: %v", err)
	}

	return model.UserTwoFactorAuthID(id)
}
