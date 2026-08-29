package usecase_test

import (
	"context"
	"testing"

	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/validator"
)

// newCreateSignInTwoFactorRecoveryUsecase wires a
// CreateSignInTwoFactorRecoveryUsecase over the test's own database. The UseCase
// opens its own transaction, so a test asserts against the rows it commits.
//
// [Ja] newCreateSignInTwoFactorRecoveryUsecase はテスト専用のデータベース上に
// CreateSignInTwoFactorRecoveryUsecase を組み立てます。UseCase は自前のトランザクションを
// 開くため、テストはそれがコミットした行を検証します。
func newCreateSignInTwoFactorRecoveryUsecase(t *testing.T, db *database.DB) *usecase.CreateSignInTwoFactorRecoveryUsecase {
	t.Helper()

	userTwoFactorAuthRepo := repository.NewUserTwoFactorAuthRepository(db)
	userSessionRepo := repository.NewUserSessionRepository(db)

	return usecase.NewCreateSignInTwoFactorRecoveryUsecase(
		db.Writer,
		validator.NewSignInTwoFactorRecoveryCreateValidator(userTwoFactorAuthRepo),
		userTwoFactorAuthRepo,
		userSessionRepo,
	)
}

// seedRecoveryUser creates a committed user with enabled 2FA and the given recovery
// codes, returning its id so a UseCase test can drive the recovery challenge
// against a real, authenticatable account and assert the used code is consumed.
//
// [Ja] seedRecoveryUser は有効な 2FA と指定のリカバリーコードを持つコミット済みユーザーを
// 作成し、その id を返す。UseCase テストが実在の認証可能なアカウントからリカバリー
// チャレンジを駆動し、使用したコードが消費されることを検証できるようにする。
func seedRecoveryUser(t *testing.T, db *database.DB, recoveryCodes []string) model.UserID {
	t.Helper()

	ctx := context.Background()

	user, err := repository.NewUserRepository(db).Create(ctx, repository.CreateUserInput{
		Email:    "2fa-rc-uc@example.com",
		Atname:   testutil.UniqueAtname(db),
		Locale:   model.LocaleJa,
		TimeZone: "Asia/Tokyo",
	})
	if err != nil {
		t.Fatalf("テスト用ユーザーの作成に失敗: %v", err)
	}

	twoFactorRepo := repository.NewUserTwoFactorAuthRepository(db)
	if _, err := twoFactorRepo.Create(ctx, repository.CreateUserTwoFactorAuthInput{
		UserID: user.ID,
		Secret: testutil.DefaultBuilderTOTPSecret,
	}); err != nil {
		t.Fatalf("2 段階認証設定の作成に失敗: %v", err)
	}
	enabled, err := twoFactorRepo.Enable(ctx, user.ID, recoveryCodes)
	if err != nil {
		t.Fatalf("2 段階認証の有効化に失敗: %v", err)
	}
	if !enabled {
		t.Fatal("2 段階認証を有効化できなかった (未有効化の行が見つからない)")
	}
	return user.ID
}

// storedRecoveryCodes reads back the user's currently stored recovery codes, for
// asserting a used code was consumed (or that a rejected attempt left them intact).
//
// [Ja] storedRecoveryCodes はユーザーの現在保存されているリカバリーコードを読み戻す。
// 使用したコードが消費されたこと (または拒否された試行がそれらを無傷で残したこと) を
// 検証するために使う。
func storedRecoveryCodes(t *testing.T, db *database.DB, userID model.UserID) []string {
	t.Helper()

	twoFactorAuth, err := repository.NewUserTwoFactorAuthRepository(db).FindEnabledByUserID(context.Background(), userID)
	if err != nil {
		t.Fatalf("2 段階認証設定の取得に失敗: %v", err)
	}
	if twoFactorAuth == nil {
		t.Fatal("有効な 2 段階認証設定が見つからない")
	}
	return twoFactorAuth.RecoveryCodes
}

// TestCreateSignInTwoFactorRecoveryUsecase_Execute_Success verifies that a valid
// recovery code returns a session token, consumes exactly that code (leaving the
// others), and persists a session resolvable by the returned token.
//
// [Ja] TestCreateSignInTwoFactorRecoveryUsecase_Execute_Success は、有効な
// リカバリーコードがセッショントークンを返し、そのコードだけを消費し (他は残す)、返した
// トークンで解決できるセッションを永続化することを検証する。
func TestCreateSignInTwoFactorRecoveryUsecase_Execute_Success(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	uc := newCreateSignInTwoFactorRecoveryUsecase(t, db)
	ctx := i18n.SetLocale(context.Background(), model.LocaleJa)
	userID := seedRecoveryUser(t, db, []string{"abcd1234", "efgh5678"})

	out, err := uc.Execute(ctx, usecase.CreateSignInTwoFactorRecoveryInput{
		UserID:    userID,
		Code:      "abcd1234",
		IPAddress: "127.0.0.1",
		UserAgent: "test-user-agent",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if out == nil || out.Token == "" {
		t.Fatal("Execute() がセッショントークンを返していない")
	}

	// The used code is gone and the unused one remains (one-time use).
	//
	// [Ja] 使用したコードは消え、未使用のコードは残る (1 回使い切り)。
	remaining := storedRecoveryCodes(t, db, userID)
	if len(remaining) != 1 || remaining[0] != "efgh5678" {
		t.Errorf("残りのリカバリーコード = %v, want [efgh5678] (使用したコードのみ消費)", remaining)
	}

	// The returned token resolves to a real, signed-in session.
	//
	// [Ja] 返したトークンは実在するサインイン済みセッションに解決する。
	session, err := repository.NewUserSessionRepository(db).FindByToken(ctx, out.Token)
	if err != nil {
		t.Fatalf("セッションの取得に失敗: %v", err)
	}
	if session == nil || session.UserID != userID {
		t.Error("返したトークンで保留中ユーザーのセッションが解決できない")
	}
}

// TestCreateSignInTwoFactorRecoveryUsecase_Execute_WrongCode verifies that an
// unknown recovery code fails with a *model.ValidationError and leaves the stored
// codes intact, consuming nothing and issuing no session.
//
// [Ja] TestCreateSignInTwoFactorRecoveryUsecase_Execute_WrongCode は、未知の
// リカバリーコードが *model.ValidationError で失敗し、保存済みコードを無傷のまま残し、
// 何も消費せずセッションも発行しないことを検証する。
func TestCreateSignInTwoFactorRecoveryUsecase_Execute_WrongCode(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	uc := newCreateSignInTwoFactorRecoveryUsecase(t, db)
	ctx := i18n.SetLocale(context.Background(), model.LocaleJa)
	userID := seedRecoveryUser(t, db, []string{"abcd1234", "efgh5678"})

	out, err := uc.Execute(ctx, usecase.CreateSignInTwoFactorRecoveryInput{
		UserID:    userID,
		Code:      "zzzz9999",
		IPAddress: "127.0.0.1",
		UserAgent: "test-user-agent",
	})
	if out != nil {
		t.Error("失敗時はトークンを返すべきでない")
	}
	if ve := model.AsValidationError(err); ve == nil {
		t.Fatalf("Execute() error = %v, want *model.ValidationError", err)
	}

	remaining := storedRecoveryCodes(t, db, userID)
	if len(remaining) != 2 {
		t.Errorf("失敗時の残りリカバリーコード数 = %d, want 2 (消費されるべきでない)", len(remaining))
	}
}
