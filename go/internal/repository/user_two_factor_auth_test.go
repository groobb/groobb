package repository_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/query"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
)

// newUserTwoFactorAuthRepo builds a UserTwoFactorAuthRepository bound to the test
// transaction (via WithTx) and creates a user to own the 2FA setting, returning
// the user ID so writes roll back when the test finishes.
//
// [Ja] newUserTwoFactorAuthRepo はテスト用トランザクションに束ねた (WithTx を通した)
// UserTwoFactorAuthRepository を作り、2FA 設定の所有ユーザーを作成してその ID を返す。
// テスト終了時に書き込みはロールバックされる。
func newUserTwoFactorAuthRepo(t *testing.T) (*repository.UserTwoFactorAuthRepository, model.UserID, context.Context) {
	t.Helper()
	db, tx := testutil.SetupTx(t)
	userID := testutil.NewUserBuilder(t, tx).Build()
	repo := repository.NewUserTwoFactorAuthRepository(query.New(db)).WithTx(tx)
	return repo, userID, context.Background()
}

func TestUserTwoFactorAuthRepository_Create(t *testing.T) {
	t.Parallel()

	repo, userID, ctx := newUserTwoFactorAuthRepo(t)

	twoFactorAuth, err := repo.Create(ctx, repository.CreateUserTwoFactorAuthInput{
		UserID: userID,
		Secret: "JBSWY3DPEHPK3PXP",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if twoFactorAuth.ID == (model.UserTwoFactorAuthID{}) {
		t.Error("Create() twoFactorAuth.ID は DB 採番で空でないはず")
	}
	if twoFactorAuth.UserID != userID {
		t.Errorf("twoFactorAuth.UserID = %v, want %v", twoFactorAuth.UserID, userID)
	}
	if twoFactorAuth.Secret != "JBSWY3DPEHPK3PXP" {
		t.Errorf("twoFactorAuth.Secret = %q, want %q", twoFactorAuth.Secret, "JBSWY3DPEHPK3PXP")
	}
	if twoFactorAuth.Enabled {
		t.Error("Create() 直後の twoFactorAuth.Enabled は false のはず")
	}
	if twoFactorAuth.EnabledAt != nil {
		t.Errorf("Create() 直後の twoFactorAuth.EnabledAt = %v, want nil", twoFactorAuth.EnabledAt)
	}
	if len(twoFactorAuth.RecoveryCodes) != 0 {
		t.Errorf("Create() 直後の twoFactorAuth.RecoveryCodes = %v, want empty", twoFactorAuth.RecoveryCodes)
	}
	if twoFactorAuth.CreatedAt.IsZero() {
		t.Error("twoFactorAuth.CreatedAt は DB 既定値で設定されるはず")
	}
	if twoFactorAuth.UpdatedAt.IsZero() {
		t.Error("twoFactorAuth.UpdatedAt は DB 既定値で設定されるはず")
	}
}

func TestUserTwoFactorAuthRepository_FindByUserID(t *testing.T) {
	t.Parallel()

	repo, userID, ctx := newUserTwoFactorAuthRepo(t)

	if _, err := repo.Create(ctx, repository.CreateUserTwoFactorAuthInput{
		UserID: userID,
		Secret: "FINDABLESECRET23",
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	t.Run("ユーザー ID で 2FA 設定を取得できる", func(t *testing.T) {
		twoFactorAuth, err := repo.FindByUserID(ctx, userID)
		if err != nil {
			t.Fatalf("FindByUserID() error = %v", err)
		}
		if twoFactorAuth == nil {
			t.Fatal("FindByUserID() = nil, want setting")
		}
		if twoFactorAuth.UserID != userID {
			t.Errorf("twoFactorAuth.UserID = %v, want %v", twoFactorAuth.UserID, userID)
		}
		if twoFactorAuth.Secret != "FINDABLESECRET23" {
			t.Errorf("twoFactorAuth.Secret = %q, want %q", twoFactorAuth.Secret, "FINDABLESECRET23")
		}
	})

	t.Run("2FA 設定を持たない user_id は (nil, nil) を返す", func(t *testing.T) {
		twoFactorAuth, err := repo.FindByUserID(ctx, model.UserID(uuid.New()))
		if err != nil {
			t.Fatalf("FindByUserID() error = %v, want nil", err)
		}
		if twoFactorAuth != nil {
			t.Errorf("FindByUserID() = %v, want nil", twoFactorAuth)
		}
	})
}

// TestUserTwoFactorAuthRepository_FindEnabledByUserID verifies that
// FindEnabledByUserID returns a setting only once it is enabled, treating a
// not-yet-enabled row the same as none.
//
// [Ja] TestUserTwoFactorAuthRepository_FindEnabledByUserID は、FindEnabledByUserID が
// 有効化された設定のみを返し、未有効化の行を設定なしと同じ扱いにすることを検証する。
func TestUserTwoFactorAuthRepository_FindEnabledByUserID(t *testing.T) {
	t.Parallel()

	repo, userID, ctx := newUserTwoFactorAuthRepo(t)

	if _, err := repo.Create(ctx, repository.CreateUserTwoFactorAuthInput{
		UserID: userID,
		Secret: "ENABLEDLOOKUP234",
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	t.Run("未有効化の間は (nil, nil) を返す", func(t *testing.T) {
		twoFactorAuth, err := repo.FindEnabledByUserID(ctx, userID)
		if err != nil {
			t.Fatalf("FindEnabledByUserID() error = %v", err)
		}
		if twoFactorAuth != nil {
			t.Errorf("FindEnabledByUserID() = %v, want nil (まだ有効化されていない)", twoFactorAuth)
		}
	})

	t.Run("有効化後は設定を返す", func(t *testing.T) {
		if _, err := repo.Enable(ctx, userID, []string{"code-a", "code-b"}); err != nil {
			t.Fatalf("Enable() error = %v", err)
		}

		twoFactorAuth, err := repo.FindEnabledByUserID(ctx, userID)
		if err != nil {
			t.Fatalf("FindEnabledByUserID() error = %v", err)
		}
		if twoFactorAuth == nil {
			t.Fatal("FindEnabledByUserID() = nil, want enabled setting")
		}
		if !twoFactorAuth.Enabled {
			t.Error("twoFactorAuth.Enabled = false, want true")
		}
	})
}

// TestUserTwoFactorAuthRepository_Enable verifies that Enable flips the setting on,
// stamps enabled_at, and stores the recovery codes.
//
// [Ja] TestUserTwoFactorAuthRepository_Enable は、Enable が設定を有効にし、enabled_at を
// 打刻し、リカバリーコードを保存することを検証する。
func TestUserTwoFactorAuthRepository_Enable(t *testing.T) {
	t.Parallel()

	repo, userID, ctx := newUserTwoFactorAuthRepo(t)

	if _, err := repo.Create(ctx, repository.CreateUserTwoFactorAuthInput{
		UserID: userID,
		Secret: "ENABLEMESECRET34",
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	recoveryCodes := []string{"aaaa1111", "bbbb2222", "cccc3333"}
	enabled, err := repo.Enable(ctx, userID, recoveryCodes)
	if err != nil {
		t.Fatalf("Enable() error = %v", err)
	}
	if !enabled {
		t.Fatal("Enable() = false, want true (未有効化の行を有効化したはず)")
	}

	twoFactorAuth, err := repo.FindByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("FindByUserID() error = %v", err)
	}
	if twoFactorAuth == nil {
		t.Fatal("FindByUserID() = nil, want setting")
	}
	if !twoFactorAuth.Enabled {
		t.Error("twoFactorAuth.Enabled = false, want true")
	}
	if twoFactorAuth.EnabledAt == nil {
		t.Error("twoFactorAuth.EnabledAt は Enable() で設定されるはず")
	}
	if len(twoFactorAuth.RecoveryCodes) != len(recoveryCodes) {
		t.Fatalf("len(twoFactorAuth.RecoveryCodes) = %d, want %d", len(twoFactorAuth.RecoveryCodes), len(recoveryCodes))
	}
	for i, code := range recoveryCodes {
		if twoFactorAuth.RecoveryCodes[i] != code {
			t.Errorf("twoFactorAuth.RecoveryCodes[%d] = %q, want %q", i, twoFactorAuth.RecoveryCodes[i], code)
		}
	}
}

// TestUserTwoFactorAuthRepository_EnableGuardsAlreadyEnabled verifies that Enable is
// guarded by enabled = false: a second Enable on an already-enabled setting matches
// no row, so it returns false (not an error) and leaves the stored recovery codes
// untouched. This is what stops a concurrent second enable from overwriting the codes
// shown to the first request.
//
// [Ja] TestUserTwoFactorAuthRepository_EnableGuardsAlreadyEnabled は Enable が
// enabled = false でガードされていることを検証する。既に有効な設定への 2 回目の Enable は
// 行に一致しないため、(エラーではなく) false を返し、保存済みのリカバリーコードを変更しない。
// これにより、同時の 2 回目の有効化が 1 回目のリクエストに表示したコードを上書きするのを防ぐ。
func TestUserTwoFactorAuthRepository_EnableGuardsAlreadyEnabled(t *testing.T) {
	t.Parallel()

	repo, userID, ctx := newUserTwoFactorAuthRepo(t)

	if _, err := repo.Create(ctx, repository.CreateUserTwoFactorAuthInput{
		UserID: userID,
		Secret: "GUARDENABLESEC56",
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	firstCodes := []string{"first-1", "first-2"}
	enabled, err := repo.Enable(ctx, userID, firstCodes)
	if err != nil {
		t.Fatalf("1 回目の Enable() error = %v", err)
	}
	if !enabled {
		t.Fatal("1 回目の Enable() = false, want true (未有効化の行を有効化するはず)")
	}

	// A second Enable finds no not-yet-enabled row (enabled = false guard), so it
	// reports false without error and does not overwrite the stored codes.
	//
	// [Ja] 2 回目の Enable は未有効化の行を見つけられず (enabled = false ガード)、
	// エラーなしで false を報告し、保存済みコードを上書きしない。
	enabledAgain, err := repo.Enable(ctx, userID, []string{"second-1", "second-2"})
	if err != nil {
		t.Fatalf("2 回目の Enable() error = %v, want nil", err)
	}
	if enabledAgain {
		t.Error("2 回目の Enable() = true, want false (既に有効な行は再有効化しない)")
	}

	stored, err := repo.FindByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("FindByUserID() error = %v", err)
	}
	if stored == nil {
		t.Fatal("FindByUserID() = nil, want setting")
	}
	if len(stored.RecoveryCodes) != len(firstCodes) {
		t.Fatalf("len(stored.RecoveryCodes) = %d, want %d (上書きされていないはず)", len(stored.RecoveryCodes), len(firstCodes))
	}
	for i, code := range firstCodes {
		if stored.RecoveryCodes[i] != code {
			t.Errorf("stored.RecoveryCodes[%d] = %q, want %q (2 回目の Enable で上書きされていない)", i, stored.RecoveryCodes[i], code)
		}
	}
}

// TestUserTwoFactorAuthRepository_UpdateRecoveryCodes verifies that
// UpdateRecoveryCodes replaces the stored codes, as when a used code is consumed
// during sign-in.
//
// [Ja] TestUserTwoFactorAuthRepository_UpdateRecoveryCodes は、UpdateRecoveryCodes が
// 保存済みコードを置き換えることを検証する (サインイン時に使用済みコードを消費する場合など)。
func TestUserTwoFactorAuthRepository_UpdateRecoveryCodes(t *testing.T) {
	t.Parallel()

	repo, userID, ctx := newUserTwoFactorAuthRepo(t)

	if _, err := repo.Create(ctx, repository.CreateUserTwoFactorAuthInput{
		UserID: userID,
		Secret: "UPDATECODESEC345",
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := repo.Enable(ctx, userID, []string{"keep-1", "used-2", "keep-3"}); err != nil {
		t.Fatalf("Enable() error = %v", err)
	}

	// Simulate consuming "used-2": write back only the remaining codes.
	//
	// [Ja] "used-2" の消費を模倣し、残りのコードだけを書き戻す。
	remaining := []string{"keep-1", "keep-3"}
	if err := repo.UpdateRecoveryCodes(ctx, userID, remaining); err != nil {
		t.Fatalf("UpdateRecoveryCodes() error = %v", err)
	}

	twoFactorAuth, err := repo.FindByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("FindByUserID() error = %v", err)
	}
	if twoFactorAuth == nil {
		t.Fatal("FindByUserID() = nil, want setting")
	}
	if len(twoFactorAuth.RecoveryCodes) != len(remaining) {
		t.Fatalf("len(twoFactorAuth.RecoveryCodes) = %d, want %d", len(twoFactorAuth.RecoveryCodes), len(remaining))
	}
	for i, code := range remaining {
		if twoFactorAuth.RecoveryCodes[i] != code {
			t.Errorf("twoFactorAuth.RecoveryCodes[%d] = %q, want %q", i, twoFactorAuth.RecoveryCodes[i], code)
		}
	}
}

// TestUserTwoFactorAuthRepository_Delete verifies that Delete removes the setting
// and that deleting when none exists is not an error (disabling is idempotent).
//
// [Ja] TestUserTwoFactorAuthRepository_Delete は、Delete が設定を削除すること、および
// 設定が無いときの削除がエラーにならないこと (無効化が冪等であること) を検証する。
func TestUserTwoFactorAuthRepository_Delete(t *testing.T) {
	t.Parallel()

	repo, userID, ctx := newUserTwoFactorAuthRepo(t)

	if _, err := repo.Create(ctx, repository.CreateUserTwoFactorAuthInput{
		UserID: userID,
		Secret: "DELETEMESECRET45",
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if err := repo.Delete(ctx, userID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	twoFactorAuth, err := repo.FindByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("FindByUserID() error = %v", err)
	}
	if twoFactorAuth != nil {
		t.Errorf("Delete() 後の FindByUserID() = %v, want nil", twoFactorAuth)
	}

	if err := repo.Delete(ctx, userID); err != nil {
		t.Errorf("設定が無い状態での Delete() error = %v, want nil", err)
	}
}

// TestUserTwoFactorAuthRepository_CreateOnConflictReturnsNil verifies that Create is
// ON CONFLICT (user_id) DO NOTHING: a second Create for a user who already has a
// setting inserts nothing, returns (nil, nil) rather than a unique-violation error,
// and leaves the existing row (and its secret) untouched. This is what lets
// PrepareTwoFactorAuthUsecase stay idempotent when concurrent first-time setup
// requests race to insert.
//
// [Ja] TestUserTwoFactorAuthRepository_CreateOnConflictReturnsNil は Create が
// ON CONFLICT (user_id) DO NOTHING であることを検証する。設定を既に持つユーザーへの 2 回目の
// Create は何も挿入せず、unique 違反エラーではなく (nil, nil) を返し、既存の行 (とその secret)
// を変更しない。これにより、同時の初回設定リクエストが挿入を競っても
// PrepareTwoFactorAuthUsecase が冪等でいられる。
func TestUserTwoFactorAuthRepository_CreateOnConflictReturnsNil(t *testing.T) {
	t.Parallel()

	repo, userID, ctx := newUserTwoFactorAuthRepo(t)

	if _, err := repo.Create(ctx, repository.CreateUserTwoFactorAuthInput{
		UserID: userID,
		Secret: "FIRSTSECRET23456",
	}); err != nil {
		t.Fatalf("1 回目の Create() error = %v", err)
	}

	second, err := repo.Create(ctx, repository.CreateUserTwoFactorAuthInput{
		UserID: userID,
		Secret: "SECONDSECRET3456",
	})
	if err != nil {
		t.Fatalf("2 回目の Create() error = %v, want nil (ON CONFLICT DO NOTHING)", err)
	}
	if second != nil {
		t.Errorf("2 回目の Create() = %v, want nil (競合時は行を返さない)", second)
	}

	// The existing row is untouched: its secret is still the first one, not the
	// second Create's secret.
	//
	// [Ja] 既存の行はそのまま: secret は 2 回目のものではなく最初のもののまま。
	stored, err := repo.FindByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("FindByUserID() error = %v", err)
	}
	if stored == nil {
		t.Fatal("FindByUserID() = nil, want 最初の設定")
	}
	if stored.Secret != "FIRSTSECRET23456" {
		t.Errorf("stored.Secret = %q, want %q (競合は既存行を上書きしない)", stored.Secret, "FIRSTSECRET23456")
	}
}
