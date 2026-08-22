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

// newSettingsEmailConfirmationHandler wires a settings_email_confirmation Handler
// over the test database's repositories, returning the email-confirmation
// repository so a test can seed a pending email-change confirmation.
//
// [Ja] newSettingsEmailConfirmationHandler はテスト用データベースのリポジトリで
// settings_email_confirmation Handler を組み立て、テストが保留中のメール変更確認を
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

// seedConfirmUser creates a committed user with the given email, returning the user
// model so a Create test can place it in the request context (as RequireAuth would)
// and drive a confirmation from it. No password is created: the confirm step
// verifies a code, not a password.
//
// [Ja] seedConfirmUser は指定 email を持つコミット済みユーザーを作成し、ユーザーモデルを
// 返す。Create テストが (RequireAuth がするように) それをリクエスト context に載せ、
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

// postConfirmation builds a POST /settings/email/confirmation request carrying the
// code as form data, with the user in the context (as RequireAuth would place it)
// and the locale set.
//
// [Ja] postConfirmation はコードをフォームデータとして運ぶ POST
// /settings/email/confirmation リクエストを組み立て、(RequireAuth が置くように) ユーザーを
// context に載せ、ロケールを設定する。
func postConfirmation(user *model.User, code, locale string) *http.Request {
	form := url.Values{"code": {code}}
	req := httptest.NewRequest(http.MethodPost, "/settings/email/confirmation", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	ctx := i18n.SetLocale(req.Context(), locale)
	ctx = middleware.SetUserToContext(ctx, user)
	return req.WithContext(ctx)
}

// decodeFlash reads and decodes the flash cookie from the response, mirroring the
// base64-encoded JSON that FlashManager writes. It fails the test if the cookie is
// missing or malformed.
//
// [Ja] decodeFlash はレスポンスのフラッシュ Cookie を読み取ってデコードする。
// FlashManager が書き込む base64 エンコードされた JSON と対になる。Cookie が無い、または
// 壊れている場合はテストを失敗させる。
func decodeFlash(t *testing.T, rec *httptest.ResponseRecorder) *session.FlashMessage {
	t.Helper()

	for _, c := range rec.Result().Cookies() {
		if c.Name != session.FlashCookieName {
			continue
		}
		data, err := base64.StdEncoding.DecodeString(c.Value)
		if err != nil {
			t.Fatalf("フラッシュ Cookie の base64 デコードに失敗: %v", err)
		}
		var flash session.FlashMessage
		if err := json.Unmarshal(data, &flash); err != nil {
			t.Fatalf("フラッシュ Cookie の JSON デコードに失敗: %v", err)
		}
		return &flash
	}
	t.Fatal("フラッシュ Cookie が設定されていない")
	return nil
}

// TestCreate_Success verifies that the correct code applies the new address to the
// user's email, sets a success flash, and redirects to the settings hub.
//
// [Ja] TestCreate_Success は、正しいコードが新しいアドレスをユーザーの email に適用し、
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
	handler.Create(rec, postConfirmation(user, "123456", i18n.LangJa))

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if loc := rec.Header().Get("Location"); loc != "/settings" {
		t.Errorf("Location = %q, want %q", loc, "/settings")
	}

	flash := decodeFlash(t, rec)
	if flash.Type != session.FlashSuccess {
		t.Errorf("flash type = %q, want %q", flash.Type, session.FlashSuccess)
	}

	updated, err := userRepo.FindByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}
	if updated == nil || updated.Email != newEmail {
		t.Errorf("user.Email = %v, want %q", updated, newEmail)
	}
}

// TestCreate_WrongCode verifies that a wrong code re-renders the form with 422 and
// the incorrect-or-expired message, echoes the entered code back, and does not
// change the user's email.
//
// [Ja] TestCreate_WrongCode は、誤ったコードがフォームを 422 と不一致・期限切れの
// メッセージで再描画し、入力したコードをエコーバックし、ユーザーの email を変更しない
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
	handler.Create(rec, postConfirmation(user, "000000", i18n.LangJa))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "確認コードが正しくないか、有効期限が切れています") {
		t.Error("コード不一致のフォーム全体エラーメッセージが描画されていない")
	}
	// The form-wide error must carry role="alert" so screen readers announce it.
	//
	// [Ja] フォーム全体のエラーはスクリーンリーダーが読み上げるよう role="alert" を持つこと。
	if !strings.Contains(body, `role="alert"`) {
		t.Error("フォーム全体のエラーに role='alert' が無い")
	}
	// The entered code is echoed back so the user does not retype it.
	//
	// [Ja] 入力したコードはエコーバックされること。
	if !strings.Contains(body, `value="000000"`) {
		t.Error("入力したコードがエコーバックされていない")
	}

	updated, err := userRepo.FindByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}
	if updated == nil || updated.Email != currentEmail {
		t.Errorf("user.Email = %v, want %q (誤ったコードで変更されてはならない)", updated, currentEmail)
	}
}
