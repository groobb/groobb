package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/middleware"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

// newSiteName builds the middleware over the supplied database.
//
// [Ja] newSiteName は渡されたデータベース上にミドルウェアを構築する。
func newSiteName(db *database.DB) *middleware.SiteName {
	getCommunityUC := usecase.NewGetCommunityUsecase(repository.NewCommunityRepository(db))

	return middleware.NewSiteName(
		func(ctx context.Context) (string, error) {
			output, err := getCommunityUC.Execute(ctx)
			if err != nil {
				return "", err
			}
			if output.Community == nil {
				return "", nil
			}
			return output.Community.Name, nil
		},
		viewmodel.SetSiteName,
	)
}

// serveSiteName runs the middleware over one request and returns the site name
// the handler downstream sees, together with whether it was reached at all.
//
// [Ja] serveSiteName はミドルウェアを 1 リクエスト分実行し、後段のハンドラーが見る
// サイトの名前を、そこへ到達したかどうかと併せて返す。
func serveSiteName(siteName *middleware.SiteName) (name string, called bool) {
	handler := siteName.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		name = viewmodel.SiteNameFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/sign_in", nil))

	return name, called
}

// TestSiteName_Middleware verifies that the name of the community this instance
// hosts reaches the context of a request on a route that loads no community of
// its own, so that the page's title can end with it.
//
// [Ja] TestSiteName_Middleware は、自前ではコミュニティを読み込まないルートの
// リクエストの context に、このインスタンスが運営するコミュニティの名前が届くことを
// 検証する。これによりページのタイトルがその名前で終われる。
func TestSiteName_Middleware(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	if _, err := db.Writer.ExecContext(context.Background(), "INSERT INTO communities (id, name) VALUES (1, ?)", "ジャズ喫茶"); err != nil {
		t.Fatalf("communities への INSERT に失敗: %v", err)
	}

	name, called := serveSiteName(newSiteName(db))

	if !called {
		t.Fatal("次のハンドラーが呼ばれていない")
	}
	if name != "ジャズ喫茶" {
		t.Errorf("SiteNameFromContext() = %q, want %q", name, "ジャズ喫茶")
	}
}

// TestSiteName_Middleware_NotSetUp verifies that an instance whose community has
// not been created yet is served without a name rather than being answered with
// an error, so its pages carry their own names alone.
//
// [Ja] TestSiteName_Middleware_NotSetUp は、コミュニティがまだ作られていない
// インスタンスが、エラーで応答されるのではなく名前の無いまま配信されることを検証する。
// そのページは自身の名前だけを運ぶ。
func TestSiteName_Middleware_NotSetUp(t *testing.T) {
	t.Parallel()

	name, called := serveSiteName(newSiteName(testutil.SetupDB(t)))

	if !called {
		t.Fatal("次のハンドラーが呼ばれていない")
	}
	if name != "" {
		t.Errorf("SiteNameFromContext() = %q, want %q", name, "")
	}
}

// TestSiteName_Middleware_ReadFailure verifies that a failing read leaves the
// request without a name instead of failing it. A database that is briefly
// unreachable must not take down the pages that need no database at all, so the
// page is still rendered and only the tail of its title is missing.
//
// [Ja] TestSiteName_Middleware_ReadFailure は、読み取りの失敗がリクエストを失敗させず、
// 名前の無いまま通すことを検証する。一時的に到達できないデータベースが、データベースを
// 必要としないページまで落としてはならない。ページは変わらず描画され、欠けるのは
// タイトルの末尾だけである。
func TestSiteName_Middleware_ReadFailure(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	if err := db.Reader.Close(); err != nil {
		t.Fatalf("Reader の Close() error = %v", err)
	}

	name, called := serveSiteName(newSiteName(db))

	if !called {
		t.Fatal("次のハンドラーが呼ばれていない")
	}
	if name != "" {
		t.Errorf("SiteNameFromContext() = %q, want %q", name, "")
	}
}
