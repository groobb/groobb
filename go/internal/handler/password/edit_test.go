package password_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/handler/password"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/query"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/validator"
)

// newPasswordHandler wires a password Handler over the shared pool. The
// UpdatePasswordResetUsecase opens its own transaction, so its tests commit rows
// and use unique tokens (the test database is reset by make test) rather than the
// rolled-back transaction pattern.
//
// [Ja] newPasswordHandler は共有プールで password Handler を組み立てる。
// UpdatePasswordResetUsecase は自前のトランザクションを開くため、そのテストはロール
// バックされるトランザクションパターンではなく、行をコミットしユニークなトークンを使う
// (テスト DB は make test がリセットする)。
func newPasswordHandler(t *testing.T) *password.Handler {
	t.Helper()

	cfg := &config.Config{Env: "test"}
	db := testutil.GetTestDB()
	queries := query.New(db)
	passwordResetTokenRepo := repository.NewPasswordResetTokenRepository(queries)
	userPasswordRepo := repository.NewUserPasswordRepository(queries)

	updatePasswordResetUC := usecase.NewUpdatePasswordResetUsecase(
		db,
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
func getPasswordEdit(token, locale string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/password/edit?token="+token, nil)
	return req.WithContext(i18n.SetLocale(req.Context(), locale))
}

// TestEdit verifies that GET /password/edit returns HTTP 200 with the new-password
// form (password and password-confirmation fields, the CSRF and reset-token
// hidden fields, and the _method=PATCH override) and the localized heading for
// each supported locale.
//
// [Ja] TestEdit は、GET /password/edit が HTTP 200 と、新パスワードフォーム (password /
// password_confirmation フィールド・CSRF とリセットトークンの hidden フィールド・
// _method=PATCH オーバーライド) を、サポートする各ロケールのローカライズ済み見出しとともに
// 返すことを検証する。
func TestEdit(t *testing.T) {
	t.Parallel()

	handler := newPasswordHandler(t)

	tests := []struct {
		name        string
		locale      string
		wantHeading string
	}{
		{name: "Japanese", locale: i18n.LangJa, wantHeading: "新しいパスワードを設定"},
		{name: "English", locale: i18n.LangEn, wantHeading: "Set a new password"},
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
