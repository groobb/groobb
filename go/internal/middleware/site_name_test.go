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

// newSiteNameは渡されたデータベース上にミドルウェアを構築する。
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

// serveSiteNameはミドルウェアを1リクエスト分実行し、後段のハンドラーが見る
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

// TestSiteName_Middlewareは、自前ではコミュニティを読み込まないルートの
// リクエストのcontextに、このインスタンスが運営するコミュニティの名前が届くことを
// 検証する。これによりページのタイトルがその名前で終われる。
func TestSiteName_Middleware(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	if _, err := db.Writer.ExecContext(context.Background(), "INSERT INTO communities (id, name) VALUES (1, ?)", "ジャズ喫茶"); err != nil {
		t.Fatalf("communitiesへのINSERTに失敗: %v", err)
	}

	name, called := serveSiteName(newSiteName(db))

	if !called {
		t.Fatal("次のハンドラーが呼ばれていない")
	}
	if name != "ジャズ喫茶" {
		t.Errorf("SiteNameFromContext() = %q、期待値 = %q", name, "ジャズ喫茶")
	}
}

// TestSiteName_Middleware_NotSetUpは、コミュニティがまだ作られていない
// インスタンスが、エラーで応答されるのではなく名前の無いまま配信されることを検証する。
// そのページは自身の名前だけを運ぶ。
func TestSiteName_Middleware_NotSetUp(t *testing.T) {
	t.Parallel()

	name, called := serveSiteName(newSiteName(testutil.SetupDB(t)))

	if !called {
		t.Fatal("次のハンドラーが呼ばれていない")
	}
	if name != "" {
		t.Errorf("SiteNameFromContext() = %q、期待値 = %q", name, "")
	}
}

// TestSiteName_Middleware_ReadFailureは、読み取りの失敗がリクエストを失敗させず、
// 名前の無いまま通すことを検証する。一時的に到達できないデータベースが、データベースを
// 必要としないページまで落としてはならない。ページは変わらず描画され、欠けるのは
// タイトルの末尾だけである。
func TestSiteName_Middleware_ReadFailure(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	if err := db.Reader.Close(); err != nil {
		t.Fatalf("ReaderのClose()のエラー = %v", err)
	}

	name, called := serveSiteName(newSiteName(db))

	if !called {
		t.Fatal("次のハンドラーが呼ばれていない")
	}
	if name != "" {
		t.Errorf("SiteNameFromContext() = %q、期待値 = %q", name, "")
	}
}
