package testutil_test

import (
	"context"
	"testing"

	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
)

// TestUserTwoFactorAuthBuilder_Buildはビルダーの生INSERTを1度通し、ビルダーが
// 直接書き込む列 (secret・enabled・enabled_at・recovery_codes) を、後続フェーズで初めて
// ビルダーが使われる時ではなくここで検証する。有効化済みの設定を組み立ててリポジトリ経由で
// 読み戻し、enabledフラグ・enabled_atの打刻・既定のsecret・リカバリーコードがすべて
// 往復することを確認する。
func TestUserTwoFactorAuthBuilder_Build(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	userID := testutil.NewUserBuilder(t, db).Build()

	recoveryCodes := []string{"aaaa1111", "bbbb2222"}
	id := testutil.NewUserTwoFactorAuthBuilder(t, db).
		WithUserID(userID).
		WithEnabled(true).
		WithRecoveryCodes(recoveryCodes).
		Build()

	repo := repository.NewUserTwoFactorAuthRepository(db)
	got, err := repo.FindByUserID(context.Background(), userID)
	if err != nil {
		t.Fatalf("FindByUserID()のエラー = %v", err)
	}
	if got == nil {
		t.Fatal("FindByUserID() = nil、期待値は構築した設定")
	}

	if got.ID != id {
		t.Errorf("got.ID = %v、期待値 = %v", got.ID, id)
	}
	if !got.Enabled {
		t.Error("WithEnabled(true) はenabledをtrueにするはず")
	}
	if got.EnabledAt == nil {
		t.Error("WithEnabled(true) はenabled_atを打刻するはず")
	}
	if got.Secret != testutil.DefaultBuilderTOTPSecret {
		t.Errorf("got.Secret = %q、期待値 = %q (既定のsecret)", got.Secret, testutil.DefaultBuilderTOTPSecret)
	}
	if len(got.RecoveryCodes) != len(recoveryCodes) {
		t.Fatalf("len(got.RecoveryCodes) = %d、期待値 = %d", len(got.RecoveryCodes), len(recoveryCodes))
	}
	for i, code := range recoveryCodes {
		if got.RecoveryCodes[i] != code {
			t.Errorf("got.RecoveryCodes[%d] = %q、期待値 = %q", i, got.RecoveryCodes[i], code)
		}
	}
}
