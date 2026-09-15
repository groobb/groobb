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

// newPasswordHandlerはテスト用データベースのリポジトリでpassword Handlerを
// 組み立てる。ハンドラーテストがUpdatePasswordResetUsecaseと、それが開く
// トランザクションを実DBに対して駆動できるようにするためである。
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

// getPasswordEditはリセットトークンを ?token= クエリで運ぶGET /password/edit
// リクエストを組み立て、contextにロケールを設定する。
func getPasswordEdit(token string, locale model.Locale) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/password/edit?token="+token, nil)
	return req.WithContext(i18n.SetLocale(req.Context(), locale))
}

// TestEditは、GET /password/editがHTTP 200と、新パスワードフォーム (password /
// password_confirmationフィールド・CSRFとリセットトークンのhiddenフィールド・
// _method=PATCHオーバーライド)、Cache-Control: no-store、サポートする各ロケールの
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
		{name: "日本語", locale: model.LocaleJa, wantHeading: "新しいパスワードを設定"},
		{name: "英語", locale: model.LocaleEn, wantHeading: "Set a new password"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			handler.Edit(rec, getPasswordEdit("link-token-abc", tt.locale))

			if rec.Code != http.StatusOK {
				t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusOK)
			}
			if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
				t.Errorf("Content-Type = %q、期待値の接頭辞 = %q", got, "text/html")
			}
			if got := rec.Result().Header.Get("Cache-Control"); got != "no-store" {
				t.Errorf("Cache-Control = %q、期待値 = %q", got, "no-store")
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
					t.Errorf("レスポンスボディに %q が含まれていない", want)
				}
			}
		})
	}
}
