package account_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/session"
	"github.com/groobb/groobb/go/internal/testutil"
)

// postAccountはatnameとpasswordフィールドをフォームデータとして運ぶ
// POST /accountリクエストを組み立て、confirmationIDが空でなければ受け渡しCookieを
// 付け、contextにロケールを設定する。
func postAccount(confirmationID, atname, password, passwordConfirmation string, locale model.Locale) *http.Request {
	form := url.Values{
		"atname":                {atname},
		"password":              {password},
		"password_confirmation": {passwordConfirmation},
	}
	req := httptest.NewRequest(http.MethodPost, "/account", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if confirmationID != "" {
		req.AddCookie(&http.Cookie{Name: session.EmailConfirmationCookieName, Value: confirmationID})
	}
	return req.WithContext(i18n.SetLocale(req.Context(), locale))
}

// seedSucceededConfirmationは指定emailのサインアップ確認を作成し成功済みとして
// 打刻 (コミット) し、ハンドラーテストが検証済みの確認からアカウント作成を駆動できるよう
// そのidを返す。
func seedSucceededConfirmation(t *testing.T, db *database.DB, email string) model.EmailConfirmationID {
	t.Helper()

	ctx := context.Background()
	repo := repository.NewEmailConfirmationRepository(db)
	confirmation, err := repo.Create(ctx, repository.CreateEmailConfirmationInput{
		Email: email,
		Event: model.EmailConfirmationEventSignUp,
		Code:  "123456",
	})
	if err != nil {
		t.Fatalf("確認の作成に失敗: %v", err)
	}
	if err := repo.Succeed(ctx, confirmation.ID); err != nil {
		t.Fatalf("確認の成功打刻に失敗: %v", err)
	}
	return confirmation.ID
}

// findCookieはレスポンスから指定名のCookieを返す。無ければnil。
func findCookie(rec *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// TestCreate_Successは、検証済みの確認と有効なパスワードが、ユーザーを作成し、
// サインインさせ (セッションCookieを設定)、受け渡しCookieを消去し、トップページへ
// リダイレクトすることを検証する。
func TestCreate_Success(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	handler := newAccountHandler(t, db)
	email := "acct-h-success@example.com"
	atname := testutil.UniqueAtname(db)
	id := seedSucceededConfirmation(t, db, email)

	rec := httptest.NewRecorder()
	handler.Create(rec, postAccount(emailConfirmationToken(t, id), atname, "password123", "password123", model.LocaleJa))

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusSeeOther)
	}
	if loc := rec.Header().Get("Location"); loc != "/" {
		t.Errorf("Location = %q、期待値 = %q", loc, "/")
	}

	sessionCookie := findCookie(rec, session.CookieName)
	if sessionCookie == nil || sessionCookie.Value == "" {
		t.Error("サインイン後にセッションCookieが設定されていない")
	}
	if ecCookie := findCookie(rec, session.EmailConfirmationCookieName); ecCookie == nil || ecCookie.MaxAge >= 0 {
		t.Error("受け渡しCookieが消去されていない")
	}

	user, err := repository.NewUserRepository(db).FindByEmail(context.Background(), email)
	if err != nil {
		t.Fatalf("FindByEmail()のエラー = %v", err)
	}
	if user == nil {
		t.Fatal("アカウント作成後にユーザーが永続化されていない")
	}
	if user.Atname != atname {
		t.Errorf("永続化されたuser.Atname = %q、期待値 = %q", user.Atname, atname)
	}
}

// TestCreate_NoCookieRedirectsToSignUpは、受け渡しCookieの無いPOST /accountが
// サインアップへリダイレクトすることを検証する。アカウント作成の元となる確認が無い
// ためである。
func TestCreate_NoCookieRedirectsToSignUp(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	handler := newAccountHandler(t, db)

	rec := httptest.NewRecorder()
	handler.Create(rec, postAccount("", testutil.UniqueAtname(db), "password123", "password123", model.LocaleJa))

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusSeeOther)
	}
	if loc := rec.Header().Get("Location"); loc != "/sign_up" {
		t.Errorf("Location = %q、期待値 = %q", loc, "/sign_up")
	}
}

// TestCreate_UnsignedNumericCookieCannotCreateAccountは、成功済み確認の連番idを
// 知っていても、サーバー発行の署名なしにはアカウント作成フローを継続できないことを
// 検証します。
func TestCreate_UnsignedNumericCookieCannotCreateAccount(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	handler := newAccountHandler(t, db)
	email := "acct-h-forged@example.com"
	id := seedSucceededConfirmation(t, db, email)

	rec := httptest.NewRecorder()
	handler.Create(rec, postAccount(id.String(), testutil.UniqueAtname(db), "password123", "password123", model.LocaleJa))

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusSeeOther)
	}
	if loc := rec.Header().Get("Location"); loc != "/sign_up" {
		t.Errorf("Location = %q、期待値 = %q", loc, "/sign_up")
	}
	if findCookie(rec, session.CookieName) != nil {
		t.Error("未署名の確認IDからセッションCookieが発行されている")
	}

	user, err := repository.NewUserRepository(db).FindByEmail(context.Background(), email)
	if err != nil {
		t.Fatalf("FindByEmail()のエラー = %v", err)
	}
	if user != nil {
		t.Errorf("未署名の確認IDからユーザーが作成された: id=%s", user.ID)
	}
}

// TestCreate_ValidationErrorは、確認が検証済みでも、パスワード確認の不一致が
// フォームを422と不一致メッセージで再描画することを検証する。
func TestCreate_ValidationError(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	handler := newAccountHandler(t, db)
	email := "acct-h-badpw@example.com"
	id := seedSucceededConfirmation(t, db, email)

	rec := httptest.NewRecorder()
	handler.Create(rec, postAccount(emailConfirmationToken(t, id), testutil.UniqueAtname(db), "password123", "different456", model.LocaleJa))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusUnprocessableEntity)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "パスワードが一致しません") {
		t.Error("不一致のエラーメッセージが描画されていない")
	}
	if !strings.Contains(body, `aria-invalid="true"`) {
		t.Error("エラー時の入力欄にaria-invalid='true' が無い")
	}
}

// TestCreate_InvalidAtnameEchoesValueは、atnameのバリデーションエラー (ここでは
// 使えない文字) がフォームを422で再描画し、送信されたatnameを入力欄にエコーバック
// してユーザーが打ち直さずに済むようにし、atnameフィールドをaria-invalidにすることを
// 検証する。
func TestCreate_InvalidAtnameEchoesValue(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	handler := newAccountHandler(t, db)
	email := "acct-h-badatname@example.com"
	id := seedSucceededConfirmation(t, db, email)

	rec := httptest.NewRecorder()
	handler.Create(rec, postAccount(emailConfirmationToken(t, id), "bad-name", "password123", "password123", model.LocaleJa))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusUnprocessableEntity)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "アットネームは半角英数字とアンダースコアのみ使用できます") {
		t.Error("atnameの形式エラーメッセージが描画されていない")
	}
	if !strings.Contains(body, `value="bad-name"`) {
		t.Error("送信されたatnameがエコーバックされていない")
	}
	if !strings.Contains(body, `aria-invalid="true"`) {
		t.Error("エラー時のatname入力欄にaria-invalid='true' が無い")
	}
}

// TestCreate_StaleConfirmationRedirectsToSignUpは、検証済みの確認を指さない
// 受け渡しCookie (ここではランダムなid) が、失効したCookieを消去しサインアップの
// やり直しへリダイレクトすることを検証する。
func TestCreate_StaleConfirmationRedirectsToSignUp(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	handler := newAccountHandler(t, db)

	rec := httptest.NewRecorder()
	handler.Create(rec, postAccount(emailConfirmationToken(t, model.EmailConfirmationID(testutil.UnusedID)), testutil.UniqueAtname(db), "password123", "password123", model.LocaleJa))

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusSeeOther)
	}
	if loc := rec.Header().Get("Location"); loc != "/sign_up" {
		t.Errorf("Location = %q、期待値 = %q", loc, "/sign_up")
	}
	if ecCookie := findCookie(rec, session.EmailConfirmationCookieName); ecCookie == nil || ecCookie.MaxAge >= 0 {
		t.Error("失効した受け渡しCookieが消去されていない")
	}
}
