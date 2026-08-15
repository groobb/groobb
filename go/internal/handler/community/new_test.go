package community_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/handler/community"
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

// newCommunityHandler wires a community Handler over the shared pool. The
// CreateCommunityUsecase opens its own transaction, so these tests commit rows
// and use unique identifiers (the test database is reset by make test) rather
// than the rolled-back transaction pattern. It also returns the community
// repository so a test can look the created community up again.
//
// [Ja] newCommunityHandler は共有プールで community Handler を組み立てます。
// CreateCommunityUsecase は自前のトランザクションを開くため、これらのテストはロールバック
// されるトランザクションパターンではなく、行をコミットしユニークな識別子を使います
// (テスト DB は make test がリセットする)。作成されたコミュニティを引き直せるよう
// コミュニティのリポジトリも返します。
func newCommunityHandler(t *testing.T) (*community.Handler, *repository.CommunityRepository) {
	t.Helper()

	cfg := &config.Config{Env: "test"}
	db := testutil.GetTestDB()
	queries := query.New(db)
	communityRepo := repository.NewCommunityRepository(queries)

	createCommunityUC := usecase.NewCreateCommunityUsecase(
		db,
		validator.NewCommunityCreateValidator(communityRepo),
		communityRepo,
		repository.NewCommunityRoleRepository(queries),
		repository.NewCommunityMemberRepository(queries),
		repository.NewCommunityMemberRoleRepository(queries),
	)
	getCommunityUC := usecase.NewGetCommunityUsecase(communityRepo)
	return community.NewHandler(cfg, session.NewFlashManager(cfg), createCommunityUC, getCommunityUC), communityRepo
}

// seedCreator creates a committed user to found a community, returning it so a
// test can place it in the request context (as RequireAuth would) and create as
// that user.
//
// [Ja] seedCreator はコミュニティを作成するコミット済みユーザーを作成し、テストが
// (RequireAuth がするように) それをリクエスト context に載せ、そのユーザーとして作成を
// 駆動できるよう返す。
func seedCreator(t *testing.T) *model.User {
	t.Helper()

	userRepo := repository.NewUserRepository(query.New(testutil.GetTestDB()))
	user, err := userRepo.Create(context.Background(), repository.CreateUserInput{
		Email:    fmt.Sprintf("community-h-%s@example.com", uuid.NewString()),
		Atname:   testutil.UniqueAtname(),
		Locale:   i18n.LangJa,
		TimeZone: "Asia/Tokyo",
	})
	if err != nil {
		t.Fatalf("ユーザーの作成に失敗: %v", err)
	}
	return user
}

// uniqueIdentifier returns a random identifier that fits the validator's format
// (ASCII letters/digits/hyphen, 20 chars max), so committed rows from parallel
// tests do not collide on the communities.identifier UNIQUE constraint. The
// random part plus its separator take 13 of the 20 characters, so prefix must be
// at most 7 characters for the result to pass validation.
//
// [Ja] uniqueIdentifier はバリデーターの形式 (ASCII 英数字 / ハイフン・最大 20 文字) に
// 収まるランダムな識別子を返す。並行テストのコミット済み行が communities.identifier の
// UNIQUE 制約で衝突しないようにするためのもの。ランダム部と区切りで 20 文字のうち 13 文字を
// 使うため、結果がバリデーションを通るには prefix は最大 7 文字とする。
func uniqueIdentifier(prefix string) string {
	return prefix + "-" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
}

// getCommunityNew builds a GET /communities/new request with the user in the
// context (as RequireAuth would place it) and the locale set.
//
// [Ja] getCommunityNew は (RequireAuth が置くように) context にユーザーを載せ、ロケールを
// 設定した GET /communities/new リクエストを組み立てる。
func getCommunityNew(user *model.User, locale string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/communities/new", nil)
	ctx := middleware.SetUserToContext(req.Context(), user)
	return req.WithContext(i18n.SetLocale(ctx, locale))
}

// findCookie returns the named cookie from a recorded response, or nil when the
// response did not set it.
//
// [Ja] findCookie は記録したレスポンスから指定された Cookie を返します。
// レスポンスに無い場合は nil を返します。
func findCookie(rec *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == name {
			return cookie
		}
	}
	return nil
}

// withCSRF wraps a handler in the real CSRF middleware, so a test drives the
// same path a production request takes: on a safe request the middleware issues
// the token and places it in the context for the page to embed, and on an unsafe
// one it matches the submitted value against the cookie and places the verified
// token in the context for the re-rendered form to embed again.
//
// [Ja] withCSRF はハンドラーを実際の CSRF ミドルウェアで包み、テストが本番のリクエストと
// 同じ経路を通るようにします。safe request ではミドルウェアがトークンを発行して context に
// 置き、ページがそれを埋め込みます。unsafe request では送信された値と Cookie を照合し、
// 再描画されるフォームが埋め込み直すための検証済みトークンを context に置きます。
func withCSRF(h http.HandlerFunc) http.Handler {
	return middleware.NewCSRF(&config.Config{Env: "test"}).Middleware(h)
}

// inputTag returns the opening input tag with the given id, failing the test if
// the rendered HTML does not contain a complete tag.
//
// [Ja] inputTag は指定した id の input 開始タグを返します。描画された HTML に完全な
// タグが無い場合はテストを失敗させます。
func inputTag(t *testing.T, body, id string) string {
	t.Helper()

	start := strings.Index(body, `<input id="`+id+`"`)
	if start < 0 {
		t.Fatalf("response body does not contain input id %q", id)
	}
	rest := body[start:]
	end := strings.IndexByte(rest, '>')
	if end < 0 {
		t.Fatalf("input id %q has no closing angle bracket", id)
	}
	return rest[:end+1]
}

// TestNew verifies that GET /communities/new returns HTTP 200 with the
// community-creation form (name and identifier fields and a CSRF hidden field),
// the localized heading, labels, and field hints for each supported locale, and
// the noindex robots meta that keeps the authenticated form out of search
// indexes. The hints are checked per locale because they are where the length
// and format rules reach the user: the name field carries no maxlength (the
// attribute counts UTF-16 code units while the validator counts runes), so its
// hint is the only place the 30-character limit appears.
//
// [Ja] TestNew は、GET /communities/new が HTTP 200 と、コミュニティ作成フォーム
// (name / identifier フィールドと CSRF hidden フィールド)、サポートする各ロケールの
// ローカライズ済み見出し・ラベル・フィールドのヒント、そして認証の背後のフォームを検索
// インデックスから除外する noindex の robots メタを返すことを検証する。ヒントをロケール別に
// 確認するのは、長さと形式の規則が利用者へ届くのがそこだからである。name フィールドは
// maxlength を持たない (属性は UTF-16 コード単位を数えるがバリデーターはルーン数を数える)
// ため、30 文字という上限が現れるのはヒントだけになる。
func TestNew(t *testing.T) {
	t.Parallel()

	handler, _ := newCommunityHandler(t)
	user := seedCreator(t)

	tests := []struct {
		name         string
		locale       string
		wantHeading  string
		wantLabel    string
		wantRequired string
		wantNameHint string

		// wantIdentifierHint holds the tail of the identifier hint rather than
		// the whole message, because the English hint opens with "community's"
		// and templ escapes the apostrophe to &#39;. The tail carries the part
		// that states the rule (allowed characters and length), so matching it
		// still fails if the hint is dropped or the key falls back to its ID.
		//
		// [Ja] wantIdentifierHint はメッセージ全体ではなく識別子のヒントの末尾を持つ。
		// 英語のヒントは "community's" で始まり、templ がアポストロフィを &#39; に
		// エスケープするため。末尾は規則 (使用可能な文字と長さ) を述べる部分なので、
		// ヒントが消えたときやキーが ID にフォールバックしたときは照合が失敗する。
		wantIdentifierHint string
	}{
		{
			name:               "Japanese",
			locale:             i18n.LangJa,
			wantHeading:        "コミュニティを作る",
			wantLabel:          "コミュニティ名",
			wantRequired:       "必須",
			wantNameHint:       "30 文字以内で入力してください",
			wantIdentifierHint: "コミュニティの URL に使われます。半角英数字とハイフンが使えます (最大 20 文字)",
		},
		{
			name:               "English",
			locale:             i18n.LangEn,
			wantHeading:        "Create a community",
			wantLabel:          "Community name",
			wantRequired:       "Required",
			wantNameHint:       "Enter at most 30 characters",
			wantIdentifierHint: "Letters, numbers, and hyphens, up to 20 characters",
		},
	}

	wrappedHandler := withCSRF(handler.New)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			wrappedHandler.ServeHTTP(rec, getCommunityNew(user, tt.locale))

			if rec.Code != http.StatusOK {
				t.Errorf("status code = %d, want %d", rec.Code, http.StatusOK)
			}
			if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
				t.Errorf("Content-Type = %q, want prefix %q", got, "text/html")
			}

			body := rec.Body.String()
			csrfCookie := findCookie(rec, middleware.CSRFCookieName)
			if csrfCookie == nil || csrfCookie.Value == "" {
				t.Fatalf("CSRF Cookie %q が発行されていない", middleware.CSRFCookieName)
			}
			wants := []string{
				tt.wantHeading,
				tt.wantLabel,
				tt.wantRequired,
				tt.wantNameHint,
				tt.wantIdentifierHint,
				`action="/communities"`,
				`method="POST"`,
				fmt.Sprintf(`name="csrf_token" value="%s"`, csrfCookie.Value),
				`name="name"`,
				`name="identifier"`,
				`pattern="[A-Za-z0-9-]+"`,
				`<meta name="robots" content="noindex"`,
			}
			for _, want := range wants {
				if !strings.Contains(body, want) {
					t.Errorf("response body does not contain %q", want)
				}
			}

			nameInput := inputTag(t, body, "name")
			if strings.Contains(nameInput, "maxlength=") {
				t.Error("name フィールドに maxlength が設定されている")
			}
			if !strings.Contains(nameInput, `autocomplete="off"`) {
				t.Error(`name フィールドに autocomplete="off" が設定されていない`)
			}
			if !strings.Contains(nameInput, "autofocus") {
				t.Error("初回表示で name フィールドに autofocus が設定されていない")
			}
			identifierInput := inputTag(t, body, "identifier")
			if !strings.Contains(identifierInput, `maxlength="20"`) {
				t.Error(`identifier フィールドに maxlength="20" が設定されていない`)
			}
			if !strings.Contains(identifierInput, `autocomplete="off"`) {
				t.Error(`identifier フィールドに autocomplete="off" が設定されていない`)
			}
			if strings.Contains(identifierInput, "autofocus") {
				t.Error("初回表示で identifier フィールドに autofocus が設定されている")
			}
		})
	}
}
