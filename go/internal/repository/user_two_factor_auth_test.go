package repository_test

import (
	"context"
	"testing"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
)

// newUserTwoFactorAuthRepoはテストが所有するデータベース上に
// UserTwoFactorAuthRepositoryを作り、2FA設定の所有ユーザーを作成してそのIDを返す。
// 各テストが既存の所有者に設定を紐付けられるようにするためである。
func newUserTwoFactorAuthRepo(t *testing.T) (*repository.UserTwoFactorAuthRepository, model.UserID, context.Context) {
	t.Helper()
	db := testutil.SetupDB(t)
	userID := testutil.NewUserBuilder(t, db).Build()
	repo := repository.NewUserTwoFactorAuthRepository(db)
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
		t.Fatalf("Create()のエラー = %v", err)
	}

	if twoFactorAuth.ID == 0 {
		t.Error("Create() twoFactorAuth.IDはDB採番で空でないはず")
	}
	if twoFactorAuth.UserID != userID {
		t.Errorf("twoFactorAuth.UserID = %v、期待値 = %v", twoFactorAuth.UserID, userID)
	}
	if twoFactorAuth.Secret != "JBSWY3DPEHPK3PXP" {
		t.Errorf("twoFactorAuth.Secret = %q、期待値 = %q", twoFactorAuth.Secret, "JBSWY3DPEHPK3PXP")
	}
	if twoFactorAuth.Enabled {
		t.Error("Create() 直後のtwoFactorAuth.Enabledはfalseのはず")
	}
	if twoFactorAuth.EnabledAt != nil {
		t.Errorf("Create() 直後のtwoFactorAuth.EnabledAt = %v、期待値 = nil", twoFactorAuth.EnabledAt)
	}
	if len(twoFactorAuth.RecoveryCodes) != 0 {
		t.Errorf("Create() 直後のtwoFactorAuth.RecoveryCodes = %v、期待値は空", twoFactorAuth.RecoveryCodes)
	}
	if twoFactorAuth.CreatedAt.IsZero() {
		t.Error("twoFactorAuth.CreatedAtはDB既定値で設定されるはず")
	}
	if twoFactorAuth.UpdatedAt.IsZero() {
		t.Error("twoFactorAuth.UpdatedAtはDB既定値で設定されるはず")
	}
}

func TestUserTwoFactorAuthRepository_FindByUserID(t *testing.T) {
	t.Parallel()

	repo, userID, ctx := newUserTwoFactorAuthRepo(t)

	if _, err := repo.Create(ctx, repository.CreateUserTwoFactorAuthInput{
		UserID: userID,
		Secret: "FINDABLESECRET23",
	}); err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}

	t.Run("ユーザーIDで2FA設定を取得できる", func(t *testing.T) {
		twoFactorAuth, err := repo.FindByUserID(ctx, userID)
		if err != nil {
			t.Fatalf("FindByUserID()のエラー = %v", err)
		}
		if twoFactorAuth == nil {
			t.Fatal("FindByUserID() = nil、期待値は設定")
		}
		if twoFactorAuth.UserID != userID {
			t.Errorf("twoFactorAuth.UserID = %v、期待値 = %v", twoFactorAuth.UserID, userID)
		}
		if twoFactorAuth.Secret != "FINDABLESECRET23" {
			t.Errorf("twoFactorAuth.Secret = %q、期待値 = %q", twoFactorAuth.Secret, "FINDABLESECRET23")
		}
	})

	t.Run("2FA設定を持たないuser_idは (nil, nil) を返す", func(t *testing.T) {
		twoFactorAuth, err := repo.FindByUserID(ctx, model.UserID(testutil.UnusedID))
		if err != nil {
			t.Fatalf("FindByUserID()のエラー = %v、期待値 = nil", err)
		}
		if twoFactorAuth != nil {
			t.Errorf("FindByUserID() = %v、期待値 = nil", twoFactorAuth)
		}
	})
}

// TestUserTwoFactorAuthRepository_FindEnabledByUserIDは、FindEnabledByUserIDが
// 有効化された設定のみを返し、未有効化の行を設定なしと同じ扱いにすることを検証する。
func TestUserTwoFactorAuthRepository_FindEnabledByUserID(t *testing.T) {
	t.Parallel()

	repo, userID, ctx := newUserTwoFactorAuthRepo(t)

	if _, err := repo.Create(ctx, repository.CreateUserTwoFactorAuthInput{
		UserID: userID,
		Secret: "ENABLEDLOOKUP234",
	}); err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}

	t.Run("未有効化の間は (nil, nil) を返す", func(t *testing.T) {
		twoFactorAuth, err := repo.FindEnabledByUserID(ctx, userID)
		if err != nil {
			t.Fatalf("FindEnabledByUserID()のエラー = %v", err)
		}
		if twoFactorAuth != nil {
			t.Errorf("FindEnabledByUserID() = %v、期待値 = nil (まだ有効化されていない)", twoFactorAuth)
		}
	})

	t.Run("有効化後は設定を返す", func(t *testing.T) {
		if _, err := repo.Enable(ctx, userID, []string{"code-a", "code-b"}); err != nil {
			t.Fatalf("Enable()のエラー = %v", err)
		}

		twoFactorAuth, err := repo.FindEnabledByUserID(ctx, userID)
		if err != nil {
			t.Fatalf("FindEnabledByUserID()のエラー = %v", err)
		}
		if twoFactorAuth == nil {
			t.Fatal("FindEnabledByUserID() = nil、期待値は有効な設定")
		}
		if !twoFactorAuth.Enabled {
			t.Error("twoFactorAuth.Enabled = false、期待値 = true")
		}
	})
}

// TestUserTwoFactorAuthRepository_Enableは、Enableが設定を有効にし、enabled_atを
// 打刻し、リカバリーコードを保存することを検証する。
func TestUserTwoFactorAuthRepository_Enable(t *testing.T) {
	t.Parallel()

	repo, userID, ctx := newUserTwoFactorAuthRepo(t)

	if _, err := repo.Create(ctx, repository.CreateUserTwoFactorAuthInput{
		UserID: userID,
		Secret: "ENABLEMESECRET34",
	}); err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}

	recoveryCodes := []string{"aaaa1111", "bbbb2222", "cccc3333"}
	enabled, err := repo.Enable(ctx, userID, recoveryCodes)
	if err != nil {
		t.Fatalf("Enable()のエラー = %v", err)
	}
	if !enabled {
		t.Fatal("Enable() = false、期待値 = true (未有効化の行を有効化したはず)")
	}

	twoFactorAuth, err := repo.FindByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("FindByUserID()のエラー = %v", err)
	}
	if twoFactorAuth == nil {
		t.Fatal("FindByUserID() = nil、期待値は設定")
	}
	if !twoFactorAuth.Enabled {
		t.Error("twoFactorAuth.Enabled = false、期待値 = true")
	}
	if twoFactorAuth.EnabledAt == nil {
		t.Error("twoFactorAuth.EnabledAtはEnable() で設定されるはず")
	}
	if len(twoFactorAuth.RecoveryCodes) != len(recoveryCodes) {
		t.Fatalf("len(twoFactorAuth.RecoveryCodes) = %d、期待値 = %d", len(twoFactorAuth.RecoveryCodes), len(recoveryCodes))
	}
	for i, code := range recoveryCodes {
		if twoFactorAuth.RecoveryCodes[i] != code {
			t.Errorf("twoFactorAuth.RecoveryCodes[%d] = %q、期待値 = %q", i, twoFactorAuth.RecoveryCodes[i], code)
		}
	}
}

// TestUserTwoFactorAuthRepository_EnableGuardsAlreadyEnabledはEnableが
// enabled = falseでガードされていることを検証する。既に有効な設定への2回目のEnableは
// 行に一致しないため、(エラーではなく) falseを返し、保存済みのリカバリーコードを変更しない。
// これにより、同時の2回目の有効化が1回目のリクエストに表示したコードを上書きするのを防ぐ。
func TestUserTwoFactorAuthRepository_EnableGuardsAlreadyEnabled(t *testing.T) {
	t.Parallel()

	repo, userID, ctx := newUserTwoFactorAuthRepo(t)

	if _, err := repo.Create(ctx, repository.CreateUserTwoFactorAuthInput{
		UserID: userID,
		Secret: "GUARDENABLESEC56",
	}); err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}

	firstCodes := []string{"first-1", "first-2"}
	enabled, err := repo.Enable(ctx, userID, firstCodes)
	if err != nil {
		t.Fatalf("1回目のEnable()のエラー = %v", err)
	}
	if !enabled {
		t.Fatal("1回目のEnable() = false、期待値 = true (未有効化の行を有効化するはず)")
	}

	// 2回目のEnableは未有効化の行を見つけられず (enabled = falseガード)、
	// エラーなしでfalseを報告し、保存済みコードを上書きしない。
	enabledAgain, err := repo.Enable(ctx, userID, []string{"second-1", "second-2"})
	if err != nil {
		t.Fatalf("2回目のEnable()のエラー = %v、期待値 = nil", err)
	}
	if enabledAgain {
		t.Error("2回目のEnable() = true、期待値 = false (既に有効な行は再有効化しない)")
	}

	stored, err := repo.FindByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("FindByUserID()のエラー = %v", err)
	}
	if stored == nil {
		t.Fatal("FindByUserID() = nil、期待値は設定")
	}
	if len(stored.RecoveryCodes) != len(firstCodes) {
		t.Fatalf("len(stored.RecoveryCodes) = %d、期待値 = %d (上書きされていないはず)", len(stored.RecoveryCodes), len(firstCodes))
	}
	for i, code := range firstCodes {
		if stored.RecoveryCodes[i] != code {
			t.Errorf("stored.RecoveryCodes[%d] = %q、期待値 = %q (2回目のEnableで上書きされていない)", i, stored.RecoveryCodes[i], code)
		}
	}
}

// TestUserTwoFactorAuthRepository_UpdateRecoveryCodesは、UpdateRecoveryCodesが
// 保存済みコードを置き換えることを検証する (サインイン時に使用済みコードを消費する場合など)。
func TestUserTwoFactorAuthRepository_UpdateRecoveryCodes(t *testing.T) {
	t.Parallel()

	repo, userID, ctx := newUserTwoFactorAuthRepo(t)

	if _, err := repo.Create(ctx, repository.CreateUserTwoFactorAuthInput{
		UserID: userID,
		Secret: "UPDATECODESEC345",
	}); err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}
	if _, err := repo.Enable(ctx, userID, []string{"keep-1", "used-2", "keep-3"}); err != nil {
		t.Fatalf("Enable()のエラー = %v", err)
	}

	// "used-2" の消費を模倣し、残りのコードだけを書き戻す。
	remaining := []string{"keep-1", "keep-3"}
	if err := repo.UpdateRecoveryCodes(ctx, userID, remaining); err != nil {
		t.Fatalf("UpdateRecoveryCodes()のエラー = %v", err)
	}

	twoFactorAuth, err := repo.FindByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("FindByUserID()のエラー = %v", err)
	}
	if twoFactorAuth == nil {
		t.Fatal("FindByUserID() = nil、期待値は設定")
	}
	if len(twoFactorAuth.RecoveryCodes) != len(remaining) {
		t.Fatalf("len(twoFactorAuth.RecoveryCodes) = %d、期待値 = %d", len(twoFactorAuth.RecoveryCodes), len(remaining))
	}
	for i, code := range remaining {
		if twoFactorAuth.RecoveryCodes[i] != code {
			t.Errorf("twoFactorAuth.RecoveryCodes[%d] = %q、期待値 = %q", i, twoFactorAuth.RecoveryCodes[i], code)
		}
	}
}

// TestUserTwoFactorAuthRepository_Deleteは、Deleteが設定を削除すること、および
// 設定が無いときの削除がエラーにならないこと (無効化が冪等であること) を検証する。
func TestUserTwoFactorAuthRepository_Delete(t *testing.T) {
	t.Parallel()

	repo, userID, ctx := newUserTwoFactorAuthRepo(t)

	if _, err := repo.Create(ctx, repository.CreateUserTwoFactorAuthInput{
		UserID: userID,
		Secret: "DELETEMESECRET45",
	}); err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}

	if err := repo.Delete(ctx, userID); err != nil {
		t.Fatalf("Delete()のエラー = %v", err)
	}

	twoFactorAuth, err := repo.FindByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("FindByUserID()のエラー = %v", err)
	}
	if twoFactorAuth != nil {
		t.Errorf("Delete() 後のFindByUserID() = %v、期待値 = nil", twoFactorAuth)
	}

	if err := repo.Delete(ctx, userID); err != nil {
		t.Errorf("設定が無い状態でのDelete()のエラー = %v、期待値 = nil", err)
	}
}

// TestUserTwoFactorAuthRepository_CreateOnConflictReturnsNilはCreateが
// ON CONFLICT (user_id) DO NOTHINGであることを検証する。設定を既に持つユーザーへの2回目の
// Createは何も挿入せず、unique違反エラーではなく (nil, nil) を返し、既存の行 (とそのsecret)
// を変更しない。これにより、同時の初回設定リクエストが挿入を競っても
// PrepareTwoFactorAuthUsecaseが冪等でいられる。
func TestUserTwoFactorAuthRepository_CreateOnConflictReturnsNil(t *testing.T) {
	t.Parallel()

	repo, userID, ctx := newUserTwoFactorAuthRepo(t)

	if _, err := repo.Create(ctx, repository.CreateUserTwoFactorAuthInput{
		UserID: userID,
		Secret: "FIRSTSECRET23456",
	}); err != nil {
		t.Fatalf("1回目のCreate()のエラー = %v", err)
	}

	second, err := repo.Create(ctx, repository.CreateUserTwoFactorAuthInput{
		UserID: userID,
		Secret: "SECONDSECRET3456",
	})
	if err != nil {
		t.Fatalf("2回目のCreate()のエラー = %v、期待値 = nil (ON CONFLICT DO NOTHING)", err)
	}
	if second != nil {
		t.Errorf("2回目のCreate() = %v、期待値 = nil (競合時は行を返さない)", second)
	}

	// 既存の行はそのまま: secretは2回目のものではなく最初のもののまま。
	stored, err := repo.FindByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("FindByUserID()のエラー = %v", err)
	}
	if stored == nil {
		t.Fatal("FindByUserID() = nil、期待値は最初の設定")
	}
	if stored.Secret != "FIRSTSECRET23456" {
		t.Errorf("stored.Secret = %q、期待値 = %q (競合は既存行を上書きしない)", stored.Secret, "FIRSTSECRET23456")
	}
}
