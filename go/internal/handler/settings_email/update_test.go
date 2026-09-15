package settings_email_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/auth"
	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/dispatcher"
	"github.com/groobb/groobb/go/internal/handler/settings_email"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/middleware"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/validator"
)

// newSettingsEmailHandlerはテスト用データベースのリポジトリでsettings_email
// Handlerをフェイクのジョブインサーターで組み立て、投入内容を検証したりenqueue失敗を
// 強制したりできるようインサーターも返します。
func newSettingsEmailHandler(t *testing.T, db *database.DB) (*settings_email.Handler, *testutil.FakeJobInserter) {
	t.Helper()

	cfg := &config.Config{Env: "test"}
	userRepo := repository.NewUserRepository(db)
	userPasswordRepo := repository.NewUserPasswordRepository(db)
	emailConfirmationRepo := repository.NewEmailConfirmationRepository(db)

	inserter := &testutil.FakeJobInserter{}
	uc := usecase.NewCreateEmailChangeUsecase(
		db.Writer,
		validator.NewSettingsEmailUpdateValidator(userRepo, userPasswordRepo),
		emailConfirmationRepo,
		dispatcher.NewDispatcher(inserter),
	)
	return settings_email.NewHandler(cfg, uc), inserter
}

// seedUserWithPasswordは指定emailとパスワード "password123" を持つコミット済み
// ユーザーを作成し、ユーザーモデルを返す。Updateテストが (RequireAuthがするように) それを
// リクエストcontextに載せ、それに対して認証できるようにする。
func seedUserWithPassword(t *testing.T, db *database.DB, email string) *model.User {
	t.Helper()

	ctx := context.Background()
	userRepo := repository.NewUserRepository(db)
	userPasswordRepo := repository.NewUserPasswordRepository(db)

	user, err := userRepo.Create(ctx, repository.CreateUserInput{
		Email:    email,
		Atname:   testutil.UniqueAtname(db),
		Locale:   "ja",
		TimeZone: "Asia/Tokyo",
	})
	if err != nil {
		t.Fatalf("テスト用ユーザーの作成に失敗: %v", err)
	}
	digest, err := auth.HashPassword("password123")
	if err != nil {
		t.Fatalf("パスワードのハッシュ化に失敗: %v", err)
	}
	if _, err := userPasswordRepo.Create(ctx, repository.CreateUserPasswordInput{
		UserID:         user.ID,
		PasswordDigest: digest,
	}); err != nil {
		t.Fatalf("テスト用パスワードの作成に失敗: %v", err)
	}
	return user
}

// patchSettingsEmailは新しいemailと現在のパスワードをフォームデータとして運ぶ
// PATCH /settings/emailリクエストを組み立て、(RequireAuthが置くように) ユーザーをcontextに
// 載せ、ロケールを設定する。
func patchSettingsEmail(user *model.User, newEmail, currentPassword string, locale model.Locale) *http.Request {
	form := url.Values{
		"email":            {newEmail},
		"current_password": {currentPassword},
	}
	req := httptest.NewRequest(http.MethodPatch, "/settings/email", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	ctx := i18n.SetLocale(req.Context(), locale)
	ctx = middleware.SetUserToContext(ctx, user)
	return req.WithContext(ctx)
}

// TestUpdate_Successは、有効な新しいemailと正しい現在のパスワードが、新しい
// アドレスの確認を発行し、メールを投入し、コード入力ステップへリダイレクトすることを
// 検証する。
func TestUpdate_Success(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	handler, inserter := newSettingsEmailHandler(t, db)
	user := seedUserWithPassword(t, db, "ec-h-cur@example.com")
	newEmail := "ec-h-new@example.com"

	rec := httptest.NewRecorder()
	handler.Update(rec, patchSettingsEmail(user, newEmail, "password123", model.LocaleJa))

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusSeeOther)
	}
	if loc := rec.Header().Get("Location"); loc != "/settings/email/confirmation/new" {
		t.Errorf("Location = %q、期待値 = %q", loc, "/settings/email/confirmation/new")
	}
	if !inserter.Called {
		t.Error("確認メールが投入されていない")
	}

	active, err := repository.NewEmailConfirmationRepository(db).FindActiveEmailChangeByUserID(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("FindActiveEmailChangeByUserID()のエラー = %v", err)
	}
	if active == nil || active.Email != newEmail {
		t.Errorf("保留中のメール変更確認 = %v、期待値 = アドレス %q", active, newEmail)
	}
}

// TestUpdate_ValidationErrorは、誤った現在のパスワードがフォームを422と
// パスワード誤りのメッセージで再描画し、メールを投入せず、確認を作成しないことを
// 検証する。
func TestUpdate_ValidationError(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	handler, inserter := newSettingsEmailHandler(t, db)
	user := seedUserWithPassword(t, db, "ec-h-ve@example.com")
	newEmail := "ec-h-ve-new@example.com"

	rec := httptest.NewRecorder()
	handler.Update(rec, patchSettingsEmail(user, newEmail, "wrongpassword", model.LocaleJa))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusUnprocessableEntity)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "現在のパスワードが正しくありません") {
		t.Error("現在パスワード誤りのエラーメッセージが描画されていない")
	}
	// スクリーンリーダーがメッセージを読み上げ、入力欄に関連付けられるよう、
	// アクセシブルなエラーマークアップがメッセージに伴っていること。
	if !strings.Contains(body, `aria-invalid="true"`) {
		t.Error("エラー時の入力欄にaria-invalid='true' が無い")
	}
	// 試した新しいemailはエコーバックされること。
	if !strings.Contains(body, `value="`+newEmail+`"`) {
		t.Error("入力した新しいemailがエコーバックされていない")
	}
	if inserter.Called {
		t.Error("バリデーション失敗時に確認メールが投入された")
	}

	active, err := repository.NewEmailConfirmationRepository(db).FindActiveEmailChangeByUserID(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("FindActiveEmailChangeByUserID()のエラー = %v", err)
	}
	if active != nil {
		t.Error("バリデーション失敗時にメール変更確認が作成された")
	}
}

// TestUpdate_EnqueueFailureは、確認メールを投入できないとき、Updateがフォームを
// 500とフォーム全体のエラー (再申請導線) で再描画し、試した新しいemailをエコーバック
// することを検証する。
func TestUpdate_EnqueueFailure(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	handler, inserter := newSettingsEmailHandler(t, db)
	inserter.Err = errors.New("queue unavailable")
	user := seedUserWithPassword(t, db, "ec-h-enq@example.com")
	newEmail := "ec-h-enq-new@example.com"

	rec := httptest.NewRecorder()
	handler.Update(rec, patchSettingsEmail(user, newEmail, "password123", model.LocaleJa))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusInternalServerError)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `action="/settings/email"`) {
		t.Error("再申請フォームが再描画されていない")
	}
	if !strings.Contains(body, `role="alert"`) {
		t.Error("フォーム全体のエラーにrole='alert' が無い")
	}
	if !strings.Contains(body, "確認コードの送信に失敗しました") {
		t.Error("フォーム全体のエラーメッセージが描画されていない")
	}
	if !strings.Contains(body, `value="`+newEmail+`"`) {
		t.Error("入力した新しいemailがエコーバックされていない")
	}
}
