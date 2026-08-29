package password_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/handler/password"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/validator"
)

// newPasswordHandler wires a password Handler over the test database's
// repositories, so a handler test drives the UpdatePasswordResetUsecase and the
// transaction it opens against a real database.
//
// [Ja] newPasswordHandler はテスト用データベースのリポジトリで password Handler を
// 組み立てる。ハンドラーテストが UpdatePasswordResetUsecase と、それが開く
// トランザクションを実 DB に対して駆動できるようにするためである。
func newPasswordHandler(t *testing.T, db *database.DB) *password.Handler {
	t.Helper()

	cfg := &config.Config{Env: "test"}
	passwordResetTokenRepo := repository.NewPasswordResetTokenRepository(db)
	userPasswordRepo := repository.NewUserPasswordRepository(db)

	updatePasswordResetUC := usecase.NewUpdatePasswordResetUsecase(
		db.Writer,
		validator.NewPasswordUpdateValidator(passwordResetTokenRepo),
		passwordResetTokenRepo,
		userPasswordRepo,
	)
	return password.NewHandler(cfg, updatePasswordResetUC)
}

// getPasswordEdit builds a GET /password/edit request carrying the reset token as
// the ?token= query, with the locale set in its context.
//
// [Ja] getPasswordEdit はリセットトークンを ?token= クエリで運ぶ GET /password/edit
// リクエストを組み立て、context にロケールを設定する。
func getPasswordEdit(token string, locale model.Locale) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/password/edit?token="+token, nil)
	return req.WithContext(i18n.SetLocale(req.Context(), locale))
}

// TestEdit verifies that GET /password/edit returns HTTP 200 with the new-password
// form (password and password-confirmation fields, the CSRF and reset-token
// hidden fields, and the _method=PATCH override), Cache-Control: no-store, and
// the localized heading for each supported locale.
//
// [Ja] TestEdit は、GET /password/edit が HTTP 200 と、新パスワードフォーム (password /
// password_confirmation フィールド・CSRF とリセットトークンの hidden フィールド・
// _method=PATCH オーバーライド)、Cache-Control: no-store、サポートする各ロケールの
// ローカライズ済み見出しを返すことを検証する。
func TestEdit(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	handler := newPasswordHandler(t, db)

	tests := []struct {
		name        string
		locale      model.Locale
		wantHeading string
	}{
		{name: "Japanese", locale: model.LocaleJa, wantHeading: "新しいパスワードを設定"},
		{name: "English", locale: model.LocaleEn, wantHeading: "Set a new password"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			handler.Edit(rec, getPasswordEdit("link-token-abc", tt.locale))

			if rec.Code != http.StatusOK {
				t.Errorf("status code = %d, want %d", rec.Code, http.StatusOK)
			}
			if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
				t.Errorf("Content-Type = %q, want prefix %q", got, "text/html")
			}
			if got := rec.Result().Header.Get("Cache-Control"); got != "no-store" {
				t.Errorf("Cache-Control = %q, want %q", got, "no-store")
			}

			body := rec.Body.String()
			wants := []string{
				tt.wantHeading,
				`action="/password"`,
				`name="_method" value="PATCH"`,
				`name="csrf_token"`,
				`name="token" value="link-token-abc"`,
				`name="password"`,
				`name="password_confirmation"`,
				`type="password"`,
			}
			for _, want := range wants {
				if !strings.Contains(body, want) {
					t.Errorf("response body does not contain %q", want)
				}
			}
		})
	}
}
