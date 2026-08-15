package community_test

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

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/middleware"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/session"
)

// testCSRFToken is the token these tests carry in both the cookie and the form
// field, so the double-submit check the CSRF middleware performs passes and the
// same value is the one a re-rendered form has to embed again.
//
// [Ja] testCSRFToken はこれらのテストが Cookie とフォームフィールドの両方に載せる
// トークン。CSRF ミドルウェアが行う double-submit の検証を通し、再描画されたフォームが
// 埋め込み直すべき値がこれと同じになるようにする。
const testCSRFToken = "test-csrf-token"

// postCommunities builds a POST /communities request carrying the name and
// identifier fields as form data, with the user in the context (as RequireAuth
// would place it) and the locale set. The CSRF token rides in both the cookie
// and the form field so the request survives the CSRF middleware, which the
// tests keep in front of the handler to cover the token's round trip back into
// a re-rendered form.
//
// [Ja] postCommunities は name と identifier フィールドをフォームデータとして運ぶ
// POST /communities リクエストを組み立て、(RequireAuth が置くように) ユーザーを context に
// 載せ、ロケールを設定する。CSRF トークンは Cookie とフォームフィールドの両方に載せ、
// リクエストが CSRF ミドルウェアを通れるようにする。テストはトークンが再描画されたフォームへ
// 戻ることを検証するため、ミドルウェアをハンドラーの手前に置いたままにしている。
func postCommunities(user *model.User, name, identifier, locale string) *http.Request {
	form := url.Values{
		"csrf_token": {testCSRFToken},
		"name":       {name},
		"identifier": {identifier},
	}
	req := httptest.NewRequest(http.MethodPost, "/communities", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: middleware.CSRFCookieName, Value: testCSRFToken})
	ctx := middleware.SetUserToContext(req.Context(), user)
	return req.WithContext(i18n.SetLocale(ctx, locale))
}

// TestCreate_Success verifies that a valid submission persists the community and
// sends the creator to its page (the short /c/{identifier} path) with a success
// flash.
//
// [Ja] TestCreate_Success は、有効な送信がコミュニティを永続化し、成功フラッシュ付きで
// 作成者をその画面 (短縮パス /c/{identifier}) へ送ることを検証する。
func TestCreate_Success(t *testing.T) {
	t.Parallel()

	handler, communityRepo := newCommunityHandler(t)
	user := seedCreator(t)
	identifier := uniqueIdentifier("ok")

	rec := httptest.NewRecorder()
	withCSRF(handler.Create).ServeHTTP(rec, postCommunities(user, "テストコミュニティ", identifier, i18n.LangJa))

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	wantLocation := "/c/" + identifier
	if loc := rec.Header().Get("Location"); loc != wantLocation {
		t.Errorf("Location = %q, want %q", loc, wantLocation)
	}

	flashCookie := findCookie(rec, session.FlashCookieName)
	if flashCookie == nil || flashCookie.Value == "" {
		t.Fatal("作成成功のフラッシュ Cookie が設定されていない")
	}
	flashData, err := base64.StdEncoding.DecodeString(flashCookie.Value)
	if err != nil {
		t.Fatalf("フラッシュ Cookie の base64 デコードに失敗: %v", err)
	}
	var flash session.FlashMessage
	if err := json.Unmarshal(flashData, &flash); err != nil {
		t.Fatalf("フラッシュ Cookie の JSON デコードに失敗: %v", err)
	}
	if flash.Type != session.FlashSuccess {
		t.Errorf("flash type = %q, want %q", flash.Type, session.FlashSuccess)
	}
	wantFlashMessage := i18n.T(i18n.SetLocale(context.Background(), i18n.LangJa), "flash_community_created")
	if flash.Message != wantFlashMessage {
		t.Errorf("flash message = %q, want %q", flash.Message, wantFlashMessage)
	}

	created, err := communityRepo.FindByIdentifier(context.Background(), identifier)
	if err != nil {
		t.Fatalf("FindByIdentifier() error = %v", err)
	}
	if created == nil {
		t.Fatal("作成後にコミュニティが永続化されていない")
	}
	if created.Name != "テストコミュニティ" {
		t.Errorf("永続化された community.Name = %q, want %q", created.Name, "テストコミュニティ")
	}
}

// TestCreate_ValidationError verifies that an identifier using a disallowed
// character re-renders the form with 422 and the format message, echoes both
// submitted values back so the user only corrects the rejected field, marks the
// identifier field aria-invalid, and puts the CSRF token back into the form so
// the corrected submission is not rejected with 403.
//
// [Ja] TestCreate_ValidationError は、使えない文字を含む識別子がフォームを 422 と形式の
// メッセージで再描画し、ユーザーが弾かれたフィールドだけを直せるよう送信された値を両方
// エコーバックし、identifier フィールドを aria-invalid にし、直した送信が 403 で拒否されない
// よう CSRF トークンをフォームへ戻すことを検証する。
func TestCreate_ValidationError(t *testing.T) {
	t.Parallel()

	handler, _ := newCommunityHandler(t)
	user := seedCreator(t)

	rec := httptest.NewRecorder()
	withCSRF(handler.Create).ServeHTTP(rec, postCommunities(user, "テストコミュニティ", "bad_identifier", i18n.LangJa))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	body := rec.Body.String()
	wants := []string{
		"URL 識別子は半角英数字とハイフンのみ使用できます",
		`value="テストコミュニティ"`,
		`value="bad_identifier"`,
		`aria-invalid="true"`,
		fmt.Sprintf(`name="csrf_token" value="%s"`, testCSRFToken),
	}
	for _, want := range wants {
		if !strings.Contains(body, want) {
			t.Errorf("response body does not contain %q", want)
		}
	}

	nameInput := inputTag(t, body, "name")
	if strings.Contains(nameInput, "autofocus") {
		t.Error("identifier のみが不正な再表示で name フィールドに autofocus が設定されている")
	}
	identifierInput := inputTag(t, body, "identifier")
	if !strings.Contains(identifierInput, "autofocus") {
		t.Error("identifier のみが不正な再表示で identifier フィールドに autofocus が設定されていない")
	}
}
