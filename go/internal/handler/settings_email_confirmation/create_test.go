package settings_email_confirmation_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/dispatcher"
	"github.com/groobb/groobb/go/internal/handler/settings_email_confirmation"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/middleware"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/query"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/session"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/validator"
)

// newSettingsEmailConfirmationHandler wires a settings_email_confirmation Handler
// over the shared pool, returning the email-confirmation repository so a test can
// seed a pending email-change confirmation. The VerifyEmailChangeUsecase opens its
// own transaction, so its tests commit rows and use unique emails (the test
// database is reset by make test) rather than the rolled-back transaction pattern.
//
// [Ja] newSettingsEmailConfirmationHandler は共有プール上に
// settings_email_confirmation Handler を組み立て、テストが保留中のメール変更確認を
// 仕込めるようメール確認リポジトリを返す。VerifyEmailChangeUsecase は自前のトランザク
// ションを開くため、そのテストはロールバックされるトランザクションパターンではなく、行を
// コミットしユニークな email を使う (テスト DB は make test がリセットする)。
func newSettingsEmailConfirmationHandler(t *testing.T) (*settings_email_confirmation.Handler, *repository.EmailConfirmationRepository, *repository.UserRepository) {
	t.Helper()

	cfg := &config.Config{Env: "test"}
	queries := query.New(testutil.GetTestDB())
	emailConfirmationRepo := repository.NewEmailConfirmationRepository(queries)
	userRepo := repository.NewUserRepository(queries)
	uc := usecase.NewVerifyEmailChangeUsecase(
		testutil.GetTestDB(),
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
func seedConfirmUser(t *testing.T, email string) *model.User {
	t.Helper()

	userRepo := repository.NewUserRepository(query.New(testutil.GetTestDB()))
	user, err := userRepo.Create(context.Background(), repository.CreateUserInput{
		Email:    email,
		Atname:   testutil.UniqueAtname(),
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

	handler, repo, userRepo := newSettingsEmailConfirmationHandler(t)
	ctx := context.Background()

	user := seedConfirmUser(t, fmt.Sprintf("ec-hc-cur-%s@example.com", uuid.NewString()))
	newEmail := fmt.Sprintf("ec-hc-new-%s@example.com", uuid.NewString())
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

	handler, repo, userRepo := newSettingsEmailConfirmationHandler(t)
	ctx := context.Background()

	currentEmail := fmt.Sprintf("ec-hc-wc-cur-%s@example.com", uuid.NewString())
	user := seedConfirmUser(t, currentEmail)
	newEmail := fmt.Sprintf("ec-hc-wc-new-%s@example.com", uuid.NewString())
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
