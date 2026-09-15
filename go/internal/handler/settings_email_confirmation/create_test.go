package settings_email_confirmation_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/dispatcher"
	"github.com/groobb/groobb/go/internal/handler/settings_email_confirmation"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/middleware"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/session"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/validator"
)

// newSettingsEmailConfirmationHandlerはテスト用データベースのリポジトリで
// settings_email_confirmation Handlerを組み立て、テストが保留中のメール変更確認を
// 仕込めるようメール確認リポジトリを返す。
func newSettingsEmailConfirmationHandler(t *testing.T, db *database.DB) (*settings_email_confirmation.Handler, *repository.EmailConfirmationRepository, *repository.UserRepository) {
	t.Helper()

	cfg := &config.Config{Env: "test"}
	emailConfirmationRepo := repository.NewEmailConfirmationRepository(db)
	userRepo := repository.NewUserRepository(db)
	uc := usecase.NewVerifyEmailChangeUsecase(
		db.Writer,
		validator.NewSettingsEmailConfirmationCreateValidator(emailConfirmationRepo),
		emailConfirmationRepo,
		userRepo,
		dispatcher.NewDispatcher(&testutil.FakeJobInserter{}),
	)
	handler := settings_email_confirmation.NewHandler(cfg, session.NewFlashManager(cfg), uc)
	return handler, emailConfirmationRepo, userRepo
}

// seedConfirmUserは指定emailを持つコミット済みユーザーを作成し、ユーザーモデルを
// 返す。Createテストが (RequireAuthがするように) それをリクエストcontextに載せ、
// そこから確認を駆動できるようにする。パスワードは作らない。確認ステップはパスワードでは
// なくコードを検証するためである。
func seedConfirmUser(t *testing.T, db *database.DB, email string) *model.User {
	t.Helper()

	userRepo := repository.NewUserRepository(db)
	user, err := userRepo.Create(context.Background(), repository.CreateUserInput{
		Email:    email,
		Atname:   testutil.UniqueAtname(db),
		Locale:   "ja",
		TimeZone: "Asia/Tokyo",
	})
	if err != nil {
		t.Fatalf("テスト用ユーザーの作成に失敗: %v", err)
	}
	return user
}

// postConfirmationはコードをフォームデータとして運ぶPOST
// /settings/email/confirmationリクエストを組み立て、(RequireAuthが置くように) ユーザーを
// contextに載せ、ロケールを設定する。
func postConfirmation(user *model.User, code string, locale model.Locale) *http.Request {
	form := url.Values{"code": {code}}
	req := httptest.NewRequest(http.MethodPost, "/settings/email/confirmation", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	ctx := i18n.SetLocale(req.Context(), locale)
	ctx = middleware.SetUserToContext(ctx, user)
	return req.WithContext(ctx)
}

// decodeFlashはレスポンスのフラッシュCookieを読み取ってデコードする。
// FlashManagerが書き込むbase64エンコードされたJSONと対になる。Cookieが無い、または
// 壊れている場合はテストを失敗させる。
func decodeFlash(t *testing.T, rec *httptest.ResponseRecorder) *session.FlashMessage {
	t.Helper()

	for _, c := range rec.Result().Cookies() {
		if c.Name != session.FlashCookieName {
			continue
		}
		data, err := base64.StdEncoding.DecodeString(c.Value)
		if err != nil {
			t.Fatalf("フラッシュCookieのbase64デコードに失敗: %v", err)
		}
		var flash session.FlashMessage
		if err := json.Unmarshal(data, &flash); err != nil {
			t.Fatalf("フラッシュCookieのJSONデコードに失敗: %v", err)
		}
		return &flash
	}
	t.Fatal("フラッシュCookieが設定されていない")
	return nil
}

// TestCreate_Successは、正しいコードが新しいアドレスをユーザーのemailに適用し、
// 成功フラッシュを設定し、設定ハブへリダイレクトすることを検証する。
func TestCreate_Success(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	handler, repo, userRepo := newSettingsEmailConfirmationHandler(t, db)
	ctx := context.Background()

	user := seedConfirmUser(t, db, "ec-hc-cur@example.com")
	newEmail := "ec-hc-new@example.com"
	if _, err := repo.CreateEmailChange(ctx, repository.CreateEmailChangeInput{UserID: user.ID, Email: newEmail, Code: "123456"}); err != nil {
		t.Fatalf("メール変更確認の作成に失敗: %v", err)
	}

	rec := httptest.NewRecorder()
	handler.Create(rec, postConfirmation(user, "123456", model.LocaleJa))

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusSeeOther)
	}
	if loc := rec.Header().Get("Location"); loc != "/settings" {
		t.Errorf("Location = %q、期待値 = %q", loc, "/settings")
	}

	flash := decodeFlash(t, rec)
	if flash.Type != session.FlashSuccess {
		t.Errorf("フラッシュの種類 = %q、期待値 = %q", flash.Type, session.FlashSuccess)
	}

	updated, err := userRepo.FindByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("FindByID()のエラー = %v", err)
	}
	if updated == nil || updated.Email != newEmail {
		t.Errorf("user.Email = %v、期待値 = %q", updated, newEmail)
	}
}

// TestCreate_WrongCodeは、誤ったコードがフォームを422と不一致・期限切れの
// メッセージで再描画し、入力したコードをエコーバックし、ユーザーのemailを変更しない
// ことを検証する。
func TestCreate_WrongCode(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	handler, repo, userRepo := newSettingsEmailConfirmationHandler(t, db)
	ctx := context.Background()

	currentEmail := "ec-hc-wc-cur@example.com"
	user := seedConfirmUser(t, db, currentEmail)
	newEmail := "ec-hc-wc-new@example.com"
	if _, err := repo.CreateEmailChange(ctx, repository.CreateEmailChangeInput{UserID: user.ID, Email: newEmail, Code: "123456"}); err != nil {
		t.Fatalf("メール変更確認の作成に失敗: %v", err)
	}

	rec := httptest.NewRecorder()
	handler.Create(rec, postConfirmation(user, "000000", model.LocaleJa))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusUnprocessableEntity)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "確認コードが正しくないか、有効期限が切れています") {
		t.Error("コード不一致のフォーム全体エラーメッセージが描画されていない")
	}
	// フォーム全体のエラーはスクリーンリーダーが読み上げるようrole="alert" を持つこと。
	if !strings.Contains(body, `role="alert"`) {
		t.Error("フォーム全体のエラーにrole='alert' が無い")
	}
	// 入力したコードはエコーバックされること。
	if !strings.Contains(body, `value="000000"`) {
		t.Error("入力したコードがエコーバックされていない")
	}

	updated, err := userRepo.FindByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("FindByID()のエラー = %v", err)
	}
	if updated == nil || updated.Email != currentEmail {
		t.Errorf("user.Email = %v、期待値 = %q (誤ったコードで変更されてはならない)", updated, currentEmail)
	}
}
