package testutil_test

import (
	"context"
	"testing"

	"github.com/groobb/groobb/go/internal/query"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
)

// TestUserTwoFactorAuthBuilder_Build exercises the builder's raw INSERT once, so
// that the columns it writes directly (secret, enabled, enabled_at,
// recovery_codes) are covered here rather than only when a later phase first
// consumes the builder. It builds an enabled setting and reads it back through
// the repository, asserting the enabled flag, the enabled_at stamp, the default
// secret, and the recovery codes all round-trip.
//
// [Ja] TestUserTwoFactorAuthBuilder_Build はビルダーの生 INSERT を 1 度通し、ビルダーが
// 直接書き込む列 (secret・enabled・enabled_at・recovery_codes) を、後続フェーズで初めて
// ビルダーが使われる時ではなくここで検証する。有効化済みの設定を組み立ててリポジトリ経由で
// 読み戻し、enabled フラグ・enabled_at の打刻・既定の secret・リカバリーコードがすべて
// 往復することを確認する。
func TestUserTwoFactorAuthBuilder_Build(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	userID := testutil.NewUserBuilder(t, tx).Build()

	recoveryCodes := []string{"aaaa1111", "bbbb2222"}
	id := testutil.NewUserTwoFactorAuthBuilder(t, tx).
		WithUserID(userID).
		WithEnabled(true).
		WithRecoveryCodes(recoveryCodes).
		Build()

	repo := repository.NewUserTwoFactorAuthRepository(query.New(db)).WithTx(tx)
	got, err := repo.FindByUserID(context.Background(), userID)
	if err != nil {
		t.Fatalf("FindByUserID() error = %v", err)
	}
	if got == nil {
		t.Fatal("FindByUserID() = nil, want built setting")
	}

	if got.ID != id {
		t.Errorf("got.ID = %v, want %v", got.ID, id)
	}
	if !got.Enabled {
		t.Error("WithEnabled(true) は enabled を true にするはず")
	}
	if got.EnabledAt == nil {
		t.Error("WithEnabled(true) は enabled_at を打刻するはず")
	}
	if got.Secret != testutil.DefaultBuilderTOTPSecret {
		t.Errorf("got.Secret = %q, want %q (既定の secret)", got.Secret, testutil.DefaultBuilderTOTPSecret)
	}
	if len(got.RecoveryCodes) != len(recoveryCodes) {
		t.Fatalf("len(got.RecoveryCodes) = %d, want %d", len(got.RecoveryCodes), len(recoveryCodes))
	}
	for i, code := range recoveryCodes {
		if got.RecoveryCodes[i] != code {
			t.Errorf("got.RecoveryCodes[%d] = %q, want %q", i, got.RecoveryCodes[i], code)
		}
	}
}
