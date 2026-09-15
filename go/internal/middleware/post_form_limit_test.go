package middleware_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/middleware"
)

// formContentTypeはHTMLフォームが送信されるエンコード方式であり、投稿の
// ルートが受け付けるものである。以下のテストは、別のものが届いたときにどうなるかを
// 確かめる場合を除きこれを送る。
const formContentType = "application/x-www-form-urlencoded"

// bodyFormOfSizeはbodyフィールドだけを持つフォームを、ちょうどsizeバイトに
// なるよう組み立てて返す。テストが欲しい大きさを名指すだけで上限の両側に立てるように
// するためである。
func bodyFormOfSize(t *testing.T, size int) string {
	t.Helper()

	const prefix = "body="
	if size < len(prefix) {
		t.Fatalf("size %d がbodyフィールドに要する %d バイトより小さい", size, len(prefix))
	}

	return prefix + strings.Repeat("a", size-len(prefix))
}

// maxLengthEncodedFormは、どのフィールドも受け付ける最長の値を持つときに
// ブラウザが送るスレッド作成フォームを返す。100コードポイントのタイトルと10,000
// コードポイントの本文を、UTF-8で1つ4バイト・パーセントエンコードで1バイトあたり3文字を
// 要する絵文字で埋めたものであり、PostFormMaxBytesが賄うべき最悪の場合である。
func maxLengthEncodedForm() string {
	return url.Values{
		"title":      {strings.Repeat("😀", 100)},
		"language":   {"ja"},
		"body":       {strings.Repeat("😀", 10000)},
		"csrf_token": {strings.Repeat("A", 44)},
	}.Encode()
}

// TestPostFormLimitは、投稿を送信するルートへ何が到達し、その手前で何が追い返され
// るかを網羅する。上限に収まるフォームは解析されて渡され、大きすぎる・フォームではない・
// デコードできないものは、問題を名指すステータスでここで応答される。他のルートへの
// リクエストと、これらのルートへの他のメソッドのリクエストは手を加えずに素通しする。
// 既に他所で配信されているフォームが現在の条件を保つのはこれによる。
func TestPostFormLimit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		// methodとpathはルートを指す。methodが空のときはPOST。
		method      string
		path        string
		contentType string
		body        string
		// chunkedはchunkedのリクエストと同じく、大きさを申告せずにボディを送る。
		chunked    bool
		wantStatus int
		wantBody   string
	}{
		{
			name:        "正常系: スレッド作成のフォームを解析して後段へ渡す",
			path:        "/b/general/threads",
			contentType: formContentType,
			body:        url.Values{"title": {"お知らせ"}, "language": {"ja"}, "body": {"本文"}}.Encode(),
			wantStatus:  http.StatusOK,
			wantBody:    "本文",
		},
		{
			name:        "正常系: 返信のフォームを解析して後段へ渡す",
			path:        "/t/123/posts",
			contentType: formContentType,
			body:        url.Values{"body": {"返信です"}}.Encode(),
			wantStatus:  http.StatusOK,
			wantBody:    "返信です",
		},
		{
			name:        "正常系: charsetを伴うContent-Type",
			path:        "/t/123/posts",
			contentType: formContentType + "; charset=UTF-8",
			body:        url.Values{"body": {"返信です"}}.Encode(),
			wantStatus:  http.StatusOK,
			wantBody:    "返信です",
		},
		{
			name:        "正常系: 境界の128KiB",
			path:        "/t/123/posts",
			contentType: formContentType,
			body:        bodyFormOfSize(t, middleware.PostFormMaxBytes),
			wantStatus:  http.StatusOK,
			wantBody:    strings.Repeat("a", middleware.PostFormMaxBytes-len("body=")),
		},
		{
			name:        "正常系: 最大長のフィールドをパーセントエンコードしても収まる",
			path:        "/b/general/threads",
			contentType: formContentType,
			body:        maxLengthEncodedForm(),
			wantStatus:  http.StatusOK,
			wantBody:    strings.Repeat("😀", 10000),
		},
		{
			name:        "異常系: 128KiBを1バイト超える",
			path:        "/t/123/posts",
			contentType: formContentType,
			body:        bodyFormOfSize(t, middleware.PostFormMaxBytes+1),
			wantStatus:  http.StatusRequestEntityTooLarge,
		},
		{
			name:        "異常系: 大きさを申告せずに128KiBを超える",
			path:        "/t/123/posts",
			contentType: formContentType,
			body:        bodyFormOfSize(t, middleware.PostFormMaxBytes+1),
			chunked:     true,
			wantStatus:  http.StatusRequestEntityTooLarge,
		},
		{
			name:        "異常系: ボディの不正なパーセント符号化",
			path:        "/t/123/posts",
			contentType: formContentType,
			body:        "body=%zz",
			wantStatus:  http.StatusBadRequest,
		},
		{
			name:        "異常系: クエリ文字列の不正なパーセント符号化",
			path:        "/t/123/posts?ref=%zz",
			contentType: formContentType,
			body:        url.Values{"body": {"返信です"}}.Encode(),
			wantStatus:  http.StatusBadRequest,
		},
		{
			name:        "異常系: multipart/form-data",
			path:        "/b/general/threads",
			contentType: "multipart/form-data; boundary=------boundary",
			body:        "--------boundary--\r\n",
			wantStatus:  http.StatusUnsupportedMediaType,
		},
		{
			name:        "異常系: application/json",
			path:        "/t/123/posts",
			contentType: "application/json",
			body:        `{"body":"返信です"}`,
			wantStatus:  http.StatusUnsupportedMediaType,
		},
		{
			name:       "異常系: Content-Typeが無い",
			path:       "/t/123/posts",
			body:       url.Values{"body": {"返信です"}}.Encode(),
			wantStatus: http.StatusUnsupportedMediaType,
		},
		{
			name:        "対象外: 他のルートのフォームは制限しない",
			path:        "/sign_in",
			contentType: formContentType,
			body:        bodyFormOfSize(t, middleware.PostFormMaxBytes+1),
			wantStatus:  http.StatusOK,
			wantBody:    strings.Repeat("a", middleware.PostFormMaxBytes+1-len("body=")),
		},
		{
			name:        "対象外: 対象パスへのGET",
			method:      http.MethodGet,
			path:        "/t/123/posts",
			contentType: formContentType,
			wantStatus:  http.StatusOK,
		},
		{
			name:        "対象外: セグメントが足りないパス",
			path:        "/b/general",
			contentType: "application/json",
			body:        `{}`,
			wantStatus:  http.StatusOK,
		},
		{
			name:        "対象外: セグメントが多いパス",
			path:        "/t/123/posts/1",
			contentType: "application/json",
			body:        `{}`,
			wantStatus:  http.StatusOK,
		},
		{
			name:        "対象外: 識別子が空のパス",
			path:        "/t//posts",
			contentType: "application/json",
			body:        `{}`,
			wantStatus:  http.StatusOK,
		},
		{
			name:        "対象外: 末尾のセグメントが違うパス",
			path:        "/b/general/posts",
			contentType: "application/json",
			body:        `{}`,
			wantStatus:  http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var gotBody string
			nextCalled := false
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
				gotBody = r.PostFormValue("body")
				w.WriteHeader(http.StatusOK)
			})

			method := tt.method
			if method == "" {
				method = http.MethodPost
			}

			var body io.Reader = strings.NewReader(tt.body)
			if tt.chunked {
				// httptest.NewRequestは長さの分かるリーダーにはContentLengthを
				// 埋めるため、長さを見られないリーダーで包み、chunkedのリクエストと
				// 同じようにボディを送る。
				body = io.NopCloser(strings.NewReader(tt.body))
			}

			req := httptest.NewRequest(method, tt.path, body)
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}
			rec := httptest.NewRecorder()

			middleware.PostFormLimit(next).ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, tt.wantStatus)
			}

			wantNextCalled := tt.wantStatus == http.StatusOK
			if nextCalled != wantNextCalled {
				t.Errorf("nextの呼び出し = %t、期待値 = %t", nextCalled, wantNextCalled)
			}

			if gotBody != tt.wantBody {
				t.Errorf("bodyフィールド = %q (%d バイト)、期待値 = %q (%d バイト)", truncate(gotBody), len(gotBody), truncate(tt.wantBody), len(tt.wantBody))
			}
		})
	}
}

// truncateは失敗メッセージのためにsを短くする。最大長のフィールドの不一致が、
// その128KiBを並べるのではなく何であるかを報告するようにするためである。
func truncate(s string) string {
	const max = 32
	if len(s) <= max {
		return s
	}

	return s[:max] + "..."
}

// TestPostFormLimit_KeepsQueryOutOfPostFormは、ここで解析した値がクエリ文字列を
// そのままにしておくこと、すなわちURLにある同じ名前のフィールドが送信されたフィールドの
// 代わりにならないことを検証する。ハンドラーは送信された内容をPostFormから読む。
// 投稿者が辿ったリンクから本文が来た返信は、その人が書いていない投稿になる。
func TestPostFormLimit_KeepsQueryOutOfPostForm(t *testing.T) {
	t.Parallel()

	var gotPostForm, gotForm string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPostForm = r.PostForm.Get("body")
		gotForm = r.Form.Get("body")
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/t/123/posts?body=%E3%82%AF%E3%82%A8%E3%83%AA", strings.NewReader(url.Values{"body": {"送信された本文"}}.Encode()))
	req.Header.Set("Content-Type", formContentType)
	rec := httptest.NewRecorder()

	middleware.PostFormLimit(next).ServeHTTP(rec, req)

	if gotPostForm != "送信された本文" {
		t.Errorf("PostFormのbody = %q、期待値 = %q", gotPostForm, "送信された本文")
	}
	if gotForm != "送信された本文" {
		t.Errorf("Formのbody = %q、期待値 = %q", gotForm, "送信された本文")
	}
}

// TestPostFormLimit_BeforeCSRFAndMethodOverrideは3つのミドルウェアが登録される
// 順序を検証する。上限に収まる送信は変わらずCSRFトークンを検証され _methodも効き、
// 大きすぎる送信は拒否されたトークンとしてではなく大きすぎるものとして応答され、
// トークンの無い送信はこのミドルウェアがボディを読んだ後も拒否される。
func TestPostFormLimit_BeforeCSRFAndMethodOverride(t *testing.T) {
	t.Parallel()

	const token = "test-csrf-token"

	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantMethod string
		wantBody   string
	}{
		{
			name:       "正常系: CSRF検証を通り後段へ届く",
			body:       url.Values{"csrf_token": {token}, "body": {"返信です"}}.Encode(),
			wantStatus: http.StatusOK,
			wantMethod: http.MethodPost,
			wantBody:   "返信です",
		},
		{
			name:       "正常系: 読み込み済みのボディでも_methodが効く",
			body:       url.Values{"csrf_token": {token}, "_method": {"DELETE"}, "body": {"返信です"}}.Encode(),
			wantStatus: http.StatusOK,
			wantMethod: http.MethodDelete,
			wantBody:   "返信です",
		},
		{
			name:       "異常系: 大きすぎる送信はCSRF検証の前に413",
			body:       bodyFormOfSize(t, middleware.PostFormMaxBytes+1),
			wantStatus: http.StatusRequestEntityTooLarge,
		},
		{
			name:       "異常系: トークンの無い送信は403",
			body:       url.Values{"body": {"返信です"}}.Encode(),
			wantStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var gotMethod, gotBody string
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotMethod = r.Method
				gotBody = r.PostFormValue("body")
				w.WriteHeader(http.StatusOK)
			})

			csrf := middleware.NewCSRF(&config.Config{Env: "test"})
			handler := middleware.PostFormLimit(csrf.Middleware(middleware.MethodOverride(next)))

			req := httptest.NewRequest(http.MethodPost, "/t/123/posts", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", formContentType)
			req.AddCookie(&http.Cookie{Name: middleware.CSRFCookieName, Value: token})
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, tt.wantStatus)
			}
			if gotMethod != tt.wantMethod {
				t.Errorf("method = %q、期待値 = %q", gotMethod, tt.wantMethod)
			}
			if gotBody != tt.wantBody {
				t.Errorf("bodyフィールド = %q、期待値 = %q", truncate(gotBody), tt.wantBody)
			}
		})
	}
}

// TestPostFormLimit_RouterPathsはミドルウェアのパス選択がchiと一致することを
// 検証する。エンコードされたスラッシュは識別子の一部に留まるため、識別子がリソースを
// 指さない場合でも、CSRFがフォームを読む前にリクエストを制限する必要がある。
func TestPostFormLimit_RouterPaths(t *testing.T) {
	t.Parallel()

	const token = "test-csrf-token"

	tests := []struct {
		name        string
		contentType string
		body        string
		chunked     bool
		wantStatus  int
		wantBody    string
	}{
		{
			name:        "正常系: 対象ルートへ解析済みフォームが届く",
			contentType: formContentType,
			body:        url.Values{"body": {"返信です"}}.Encode(),
			wantStatus:  http.StatusNoContent,
			wantBody:    "返信です",
		},
		{
			name:        "異常系: Content-Length付きのサイズ超過",
			contentType: formContentType,
			body:        bodyFormOfSize(t, middleware.PostFormMaxBytes+1),
			wantStatus:  http.StatusRequestEntityTooLarge,
		},
		{
			name:        "異常系: Content-Lengthなしのサイズ超過",
			contentType: formContentType,
			body:        bodyFormOfSize(t, middleware.PostFormMaxBytes+1),
			chunked:     true,
			wantStatus:  http.StatusRequestEntityTooLarge,
		},
		{
			name:        "異常系: 不正なフォーム符号化",
			contentType: formContentType,
			body:        "body=%zz",
			wantStatus:  http.StatusBadRequest,
		},
		{
			name:        "異常系: 未対応のContent-Type",
			contentType: "application/json",
			body:        "{}",
			wantStatus:  http.StatusUnsupportedMediaType,
		},
	}

	for _, requestPath := range []string{
		"/b/general/threads",
		"/b/general%2Fextra/threads",
		"/t/123/posts",
		"/t/123%2Fextra/posts",
	} {
		t.Run(requestPath, func(t *testing.T) {
			t.Parallel()

			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					var gotBody string
					nextCalled := false
					next := func(w http.ResponseWriter, r *http.Request) {
						nextCalled = true
						gotBody = r.PostForm.Get("body")
						w.WriteHeader(http.StatusNoContent)
					}

					csrf := middleware.NewCSRF(&config.Config{Env: "test"})
					router := chi.NewRouter()
					router.Use(chimiddleware.RedirectSlashes)
					router.Use(middleware.PostFormLimit)
					router.Use(csrf.Middleware)
					router.Use(middleware.MethodOverride)
					router.Post("/b/{slug}/threads", next)
					router.Post("/t/{id}/posts", next)

					var body io.Reader = strings.NewReader(tt.body)
					if tt.chunked {
						body = io.NopCloser(body)
					}
					req := httptest.NewRequest(http.MethodPost, requestPath, body)
					req.Header.Set("Content-Type", tt.contentType)
					req.Header.Set("X-CSRF-Token", token)
					req.AddCookie(&http.Cookie{Name: middleware.CSRFCookieName, Value: token})
					rec := httptest.NewRecorder()

					router.ServeHTTP(rec, req)

					if rec.Code != tt.wantStatus {
						t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, tt.wantStatus)
					}
					wantNextCalled := tt.wantStatus == http.StatusNoContent
					if nextCalled != wantNextCalled {
						t.Errorf("nextの呼び出し = %t、期待値 = %t", nextCalled, wantNextCalled)
					}
					if gotBody != tt.wantBody {
						t.Errorf("bodyフィールド = %q、期待値 = %q", truncate(gotBody), tt.wantBody)
					}
				})
			}
		})
	}
}
