package settings_withdrawal_test

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
	"time"

	"github.com/google/uuid"

	"github.com/groobb/groobb/go/internal/auth"
	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/handler/settings_withdrawal"
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

// newSettingsWithdrawalHandler wires a settings_withdrawal Handler over the shared
// pool. The DeleteAccountUsecase opens its own transaction, so its tests commit
// rows and use unique identifiers (the test database is reset by make test) rather
// than the rolled-back transaction pattern.
//
// [Ja] newSettingsWithdrawalHandler は共有プール上に settings_withdrawal Handler を
// 組み立てます。DeleteAccountUsecase は自前のトランザクションを開くため、そのテストは
// ロールバックされるトランザクションパターンではなく、行をコミットしユニークな識別子を
// 使います (テスト DB は make test がリセットする)。
func newSettingsWithdrawalHandler(t *testing.T) *settings_withdrawal.Handler {
	t.Helper()

	cfg := &config.Config{Env: "test"}
	queries := query.New(testutil.GetTestDB())
	userRepo := repository.NewUserRepository(queries)
	userPasswordRepo := repository.NewUserPasswordRepository(queries)
	userSessionRepo := repository.NewUserSessionRepository(queries)

	sessionMgr := session.NewManager(userRepo, cfg)
	flashMgr := session.NewFlashManager(cfg)
	deleteAccountUC := usecase.NewDeleteAccountUsecase(
		testutil.GetTestDB(),
		validator.NewSettingsWithdrawalDeleteValidator(userPasswordRepo),
		userRepo,
		userSessionRepo,
	)
	return settings_withdrawal.NewHandler(cfg, sessionMgr, flashMgr, deleteAccountUC)
}

// seedWithdrawalUser creates a committed user with the password "password123" and a
// live session, returning the user model (so a test can place it in the request
// context, as RequireAuth would) and the session token (so the request can carry
// the matching session cookie).
//
// [Ja] seedWithdrawalUser はパスワード "password123" と有効なセッションを 1 つ持つ
// コミット済みユーザーを作成し、ユーザーモデル (テストが RequireAuth のように context に
// 載せられるよう) とセッショントークン (リクエストが一致するセッション Cookie を運べるよう)
// を返します。
func seedWithdrawalUser(t *testing.T) (*model.User, string) {
	t.Helper()

	ctx := context.Background()
	queries := query.New(testutil.GetTestDB())
	userRepo := repository.NewUserRepository(queries)
	userPasswordRepo := repository.NewUserPasswordRepository(queries)
	userSessionRepo := repository.NewUserSessionRepository(queries)

	user, err := userRepo.Create(ctx, repository.CreateUserInput{
		Email:    fmt.Sprintf("wd-h-%s@example.com", uuid.NewString()),
		Atname:   testutil.UniqueAtname(),
		Locale:   i18n.LangJa,
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
	token := "wd-h-token-" + uuid.NewString()
	if _, err := userSessionRepo.Create(ctx, repository.CreateUserSessionInput{
		UserID:    user.ID,
		Token:     token,
		IPAddress: "203.0.113.7",
		UserAgent: "test-agent",
	}); err != nil {
		t.Fatalf("テスト用セッションの作成に失敗: %v", err)
	}
	return user, token
}

// deleteWithdrawal builds a DELETE /settings/withdrawal request carrying the current
// password as form data, the session cookie for the token, the user in the context
// (as RequireAuth would place it), and the locale set. The form is parsed while the
// request is still a POST and only then is the method switched to DELETE, mirroring
// what the method-override middleware does in production: Go's ParseForm reads the
// body only for POST/PUT/PATCH, so a request constructed directly as DELETE would
// leave the handler's FormValue empty.
//
// [Ja] deleteWithdrawal は現在のパスワードをフォームデータとして運ぶ
// DELETE /settings/withdrawal リクエストを組み立て、token のセッション Cookie、
// (RequireAuth が置くように) context のユーザー、そして設定したロケールを載せます。
// フォームはリクエストがまだ POST のうちに解析し、その後でメソッドを DELETE に切り替えます。
// これは本番のメソッドオーバーライドミドルウェアの挙動を再現したものです。Go の ParseForm は
// POST/PUT/PATCH のときだけボディを読むため、最初から DELETE として組み立てたリクエストでは
// ハンドラーの FormValue が空になってしまいます。
func deleteWithdrawal(user *model.User, currentPassword, token, locale string) *http.Request {
	form := url.Values{"current_password": {currentPassword}}
	req := httptest.NewRequest(http.MethodPost, "/settings/withdrawal", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err := req.ParseForm(); err != nil {
		panic(err)
	}
	req.Method = http.MethodDelete
	req.AddCookie(&http.Cookie{Name: session.CookieName, Value: token})
	ctx := i18n.SetLocale(req.Context(), locale)
	ctx = middleware.SetUserToContext(ctx, user)
	return req.WithContext(ctx)
}

// findCookie returns the cookie with the given name from the response, or nil.
//
// [Ja] findCookie はレスポンスから指定名の Cookie を返す。無ければ nil。
func findCookie(rec *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// decodeFlash reads and decodes the flash cookie from the response, mirroring the
// base64-encoded JSON that FlashManager writes. It fails the test if the cookie is
// missing or malformed.
//
// [Ja] decodeFlash はレスポンスのフラッシュ Cookie を読み取ってデコードする。
// FlashManager が書き込む base64 エンコードされた JSON と対になる。Cookie が無い、
// または壊れている場合はテストを失敗させる。
func decodeFlash(t *testing.T, rec *httptest.ResponseRecorder) *session.FlashMessage {
	t.Helper()

	c := findCookie(rec, session.FlashCookieName)
	if c == nil {
		t.Fatal("フラッシュ Cookie が設定されていない")
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

// TestDelete_Success verifies that DELETE /settings/withdrawal with the correct
// current password withdraws the account (soft-deletes and anonymizes the user and
// deletes its sessions), clears the session cookie, sets the completion flash, and
// redirects to the top page.
//
// [Ja] TestDelete_Success は、正しい現在のパスワード付きの DELETE /settings/withdrawal が
// アカウントを退会させ (ユーザーを論理削除・匿名化し、そのセッションを削除する)、
// セッション Cookie を消去し、完了フラッシュを設定し、トップページへリダイレクトすることを
// 検証する。
func TestDelete_Success(t *testing.T) {
	t.Parallel()

	handler := newSettingsWithdrawalHandler(t)
	user, token := seedWithdrawalUser(t)

	rec := httptest.NewRecorder()
	handler.Delete(rec, deleteWithdrawal(user, "password123", token, i18n.LangJa))

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if loc := rec.Header().Get("Location"); loc != "/" {
		t.Errorf("Location = %q, want %q", loc, "/")
	}

	// The session cookie is cleared (a matching cookie with MaxAge < 0).
	//
	// [Ja] セッション Cookie が消去される (MaxAge < 0 の同名 Cookie)。
	if c := findCookie(rec, session.CookieName); c == nil || c.MaxAge >= 0 {
		t.Error("セッション Cookie が消去されていない")
	}

	// A success flash is set so the top page renders the "withdrawn" toast.
	//
	// [Ja] トップページが「退会しました」toast を描画するよう成功フラッシュが設定される。
	flash := decodeFlash(t, rec)
	if flash.Type != session.FlashSuccess {
		t.Errorf("flash type = %q, want %q", flash.Type, session.FlashSuccess)
	}
	if want := i18n.T(i18n.SetLocale(context.Background(), i18n.LangJa), "flash_account_withdrawn"); flash.Message != want {
		t.Errorf("flash message = %q, want %q", flash.Message, want)
	}

	// The user row is soft-deleted. The row is queried directly (not via a lookup
	// that filters deleted_at) so the soft-deleted user is still observable.
	//
	// [Ja] ユーザー行が論理削除される。行は (deleted_at で絞るルックアップではなく) 直接
	// クエリするため、論理削除されたユーザーも観測できる。
	var deletedAt *time.Time
	if err := testutil.GetTestDB().QueryRow(context.Background(),
		`SELECT deleted_at FROM users WHERE id = $1`, uuid.UUID(user.ID),
	).Scan(&deletedAt); err != nil {
		t.Fatalf("退会後のユーザー行の取得に失敗: %v", err)
	}
	if deletedAt == nil {
		t.Error("deleted_at がセットされていない (論理削除されていない)")
	}

	// The session row is gone (all devices signed out by the UseCase).
	//
	// [Ja] セッション行が消えている (UseCase が全端末をサインアウトさせた)。
	if got := countUserSessions(t, user.ID); got != 0 {
		t.Errorf("退会後のセッション数 = %d, want 0", got)
	}
}

// TestDelete_ValidationError verifies that a wrong current password re-renders the
// confirmation form with 422 and the incorrect-password message, and leaves the
// account fully intact: not soft-deleted and with its sessions still present.
//
// [Ja] TestDelete_ValidationError は、誤った現在のパスワードが確認フォームを 422 と
// パスワード誤りのメッセージで再描画し、アカウントを完全に無傷のまま (論理削除されず、
// セッションも残ったまま) にすることを検証する。
func TestDelete_ValidationError(t *testing.T) {
	t.Parallel()

	handler := newSettingsWithdrawalHandler(t)
	user, _ := seedWithdrawalUser(t)

	rec := httptest.NewRecorder()
	handler.Delete(rec, deleteWithdrawal(user, "wrongpassword", "wd-h-token-unused", i18n.LangJa))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "現在のパスワードが正しくありません") {
		t.Error("現在パスワード誤りのエラーメッセージが描画されていない")
	}
	// The accessible-error markup must accompany the message so screen readers
	// announce it and associate it with the input.
	//
	// [Ja] スクリーンリーダーがメッセージを読み上げ、入力欄に関連付けられるよう、
	// アクセシブルなエラーマークアップがメッセージに伴っていること。
	if !strings.Contains(body, `aria-invalid="true"`) {
		t.Error("エラー時の入力欄に aria-invalid='true' が無い")
	}
	// The confirmation form is re-rendered (still driving DELETE /settings/withdrawal).
	//
	// [Ja] 確認フォームが再描画される (引き続き DELETE /settings/withdrawal を動かす)。
	if !strings.Contains(body, `action="/settings/withdrawal"`) {
		t.Error("退会確認フォームが再描画されていない")
	}

	// The account is untouched: not soft-deleted, sessions intact.
	//
	// [Ja] アカウントは無傷: 論理削除されず、セッションも残る。
	var deletedAt *time.Time
	if err := testutil.GetTestDB().QueryRow(context.Background(),
		`SELECT deleted_at FROM users WHERE id = $1`, uuid.UUID(user.ID),
	).Scan(&deletedAt); err != nil {
		t.Fatalf("ユーザー行の取得に失敗: %v", err)
	}
	if deletedAt != nil {
		t.Error("バリデーション失敗時にユーザーが論理削除された")
	}
	if got := countUserSessions(t, user.ID); got != 1 {
		t.Errorf("バリデーション失敗時のセッション数 = %d, want 1 (削除されるべきでない)", got)
	}
}

// countUserSessions returns how many sessions the given user still owns, for
// asserting that withdrawal cleared them (or that a rejected withdrawal left them).
//
// [Ja] countUserSessions は指定ユーザーがまだ所有するセッション数を返す。退会が
// それらを消したこと (または拒否された退会がそれらを残したこと) を検証するために使う。
func countUserSessions(t *testing.T, userID model.UserID) int {
	t.Helper()

	var count int
	if err := testutil.GetTestDB().QueryRow(context.Background(),
		`SELECT count(*) FROM user_sessions WHERE user_id = $1`, uuid.UUID(userID),
	).Scan(&count); err != nil {
		t.Fatalf("セッション件数の取得に失敗: %v", err)
	}
	return count
}
