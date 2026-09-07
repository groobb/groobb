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

// formContentType is the encoding an HTML form is submitted in, which the post
// routes accept and the tests below send unless they are checking what happens
// when something else arrives.
//
// [Ja] formContentType はHTMLフォームが送信されるエンコード方式であり、投稿の
// ルートが受け付けるものである。以下のテストは、別のものが届いたときにどうなるかを
// 確かめる場合を除きこれを送る。
const formContentType = "application/x-www-form-urlencoded"

// bodyFormOfSize returns a form whose only field is a body field, encoded to
// exactly size bytes, so a test can sit on either side of the limit by naming
// the size it wants.
//
// [Ja] bodyFormOfSize は body フィールドだけを持つフォームを、ちょうど size バイトに
// なるよう組み立てて返す。テストが欲しい大きさを名指すだけで上限の両側に立てるように
// するためである。
func bodyFormOfSize(t *testing.T, size int) string {
	t.Helper()

	const prefix = "body="
	if size < len(prefix) {
		t.Fatalf("size %d is smaller than the %d bytes a body field costs", size, len(prefix))
	}

	return prefix + strings.Repeat("a", size-len(prefix))
}

// maxLengthEncodedForm returns the thread-creation form as a browser sends it
// when every field holds the longest value it accepts: a title of 100 code
// points and a body of 10,000, in emoji, which cost four bytes each in UTF-8
// and three characters per byte once percent-encoded. It is the worst case
// PostFormMaxBytes is sized for.
//
// [Ja] maxLengthEncodedForm は、どのフィールドも受け付ける最長の値を持つときに
// ブラウザが送るスレッド作成フォームを返す。100コードポイントのタイトルと10,000
// コードポイントの本文を、UTF-8で1つ4バイト・パーセントエンコードで1バイトあたり3文字を
// 要する絵文字で埋めたものであり、PostFormMaxBytes が賄うべき最悪の場合である。
func maxLengthEncodedForm() string {
	return url.Values{
		"title":      {strings.Repeat("😀", 100)},
		"language":   {"ja"},
		"body":       {strings.Repeat("😀", 10000)},
		"csrf_token": {strings.Repeat("A", 44)},
	}.Encode()
}

// TestPostFormLimit covers what reaches the routes that submit a post and what
// is turned away before them: a form within the limit is parsed and handed on,
// while one that is too large, is not a form, or cannot be decoded is answered
// here with the status that names the problem. Requests to any other route, and
// to these routes by any other method, pass through untouched, which is how the
// forms already served elsewhere keep their current bounds.
//
// [Ja] TestPostFormLimit は、投稿を送信するルートへ何が到達し、その手前で何が追い返され
// るかを網羅する。上限に収まるフォームは解析されて渡され、大きすぎる・フォームではない・
// デコードできないものは、問題を名指すステータスでここで応答される。他のルートへの
// リクエストと、これらのルートへの他のメソッドのリクエストは手を加えずに素通しする。
// 既に他所で配信されているフォームが現在の条件を保つのはこれによる。
func TestPostFormLimit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		// method and path address the route; an empty method means POST.
		//
		// [Ja] method と path はルートを指す。method が空のときは POST。
		method      string
		path        string
		contentType string
		body        string
		// chunked sends the body without announcing its size, as a chunked
		// request does.
		//
		// [Ja] chunked は chunked のリクエストと同じく、大きさを申告せずにボディを送る。
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
				// httptest.NewRequest fills in ContentLength for a reader whose
				// length it knows, so hide the reader behind one whose length it
				// cannot see to send the body the way a chunked request does.
				//
				// [Ja] httptest.NewRequest は長さの分かるリーダーには ContentLength を
				// 埋めるため、長さを見られないリーダーで包み、chunked のリクエストと
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
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}

			wantNextCalled := tt.wantStatus == http.StatusOK
			if nextCalled != wantNextCalled {
				t.Errorf("next called = %t, want %t", nextCalled, wantNextCalled)
			}

			if gotBody != tt.wantBody {
				t.Errorf("body field = %q (%d bytes), want %q (%d bytes)", truncate(gotBody), len(gotBody), truncate(tt.wantBody), len(tt.wantBody))
			}
		})
	}
}

// truncate shortens s for a failure message, so a mismatch on a field holding
// the maximum length reports what it is rather than 128 KiB of it.
//
// [Ja] truncate は失敗メッセージのために s を短くする。最大長のフィールドの不一致が、
// その128KiBを並べるのではなく何であるかを報告するようにするためである。
func truncate(s string) string {
	const max = 32
	if len(s) <= max {
		return s
	}

	return s[:max] + "..."
}

// TestPostFormLimit_KeepsQueryOutOfPostForm verifies that the values parsed
// here leave the query string where it is: a field of the same name in the URL
// does not stand in for the submitted one. The handler reads the submission
// from PostForm, and a reply whose body came from a link the poster followed
// would be a post they never wrote.
//
// [Ja] TestPostFormLimit_KeepsQueryOutOfPostForm は、ここで解析した値がクエリ文字列を
// そのままにしておくこと、すなわちURLにある同じ名前のフィールドが送信されたフィールドの
// 代わりにならないことを検証する。ハンドラーは送信された内容を PostForm から読む。
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
		t.Errorf("PostForm body = %q, want %q", gotPostForm, "送信された本文")
	}
	if gotForm != "送信された本文" {
		t.Errorf("Form body = %q, want %q", gotForm, "送信された本文")
	}
}

// TestPostFormLimit_BeforeCSRFAndMethodOverride verifies the order the three
// middlewares are registered in: a submission within the limit still has its
// CSRF token checked and its _method honored, an oversized one is answered as
// too large rather than as a rejected token, and a submission with no token is
// still refused after this middleware has read its body.
//
// [Ja] TestPostFormLimit_BeforeCSRFAndMethodOverride は3つのミドルウェアが登録される
// 順序を検証する。上限に収まる送信は変わらずCSRFトークンを検証され _method も効き、
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
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if gotMethod != tt.wantMethod {
				t.Errorf("method = %q, want %q", gotMethod, tt.wantMethod)
			}
			if gotBody != tt.wantBody {
				t.Errorf("body field = %q, want %q", truncate(gotBody), tt.wantBody)
			}
		})
	}
}

// TestPostFormLimit_RouterPaths keeps the middleware's path selection aligned
// with chi: an encoded slash remains inside an identifier, so the request must
// be bounded before CSRF reads the form even if that identifier names no resource.
//
// [Ja] TestPostFormLimit_RouterPaths はミドルウェアのパス選択がchiと一致することを
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
						t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
					}
					wantNextCalled := tt.wantStatus == http.StatusNoContent
					if nextCalled != wantNextCalled {
						t.Errorf("next called = %t, want %t", nextCalled, wantNextCalled)
					}
					if gotBody != tt.wantBody {
						t.Errorf("body field = %q, want %q", truncate(gotBody), tt.wantBody)
					}
				})
			}
		})
	}
}
