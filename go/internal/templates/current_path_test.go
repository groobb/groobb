package templates_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/groobb/groobb/go/internal/templates"
)

// TestIsCurrentPath verifies that IsCurrentPath marks only the page being
// rendered, and reports false rather than marking everything when the context
// carries no path, so a template rendered outside a request stays unmarked.
//
// [Ja] TestIsCurrentPath は IsCurrentPath が今描画しているページだけに一致し、context が
// パスを持たないときはすべてを一致とせず false を返すことを検証します。これによりリクエスト
// の外で描画するテンプレートには印が付きません。
func TestIsCurrentPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		setPath  bool
		current  string
		linkPath string
		want     bool
	}{
		{name: "matches the current page", setPath: true, current: "/home", linkPath: "/home", want: true},
		{name: "does not match another page", setPath: true, current: "/settings", linkPath: "/home", want: false},
		{name: "does not match a page below it", setPath: true, current: "/settings/email/edit", linkPath: "/settings", want: false},
		{name: "no path in the context", setPath: false, linkPath: "/home", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			if tt.setPath {
				ctx = templates.SetCurrentPath(ctx, tt.current)
			}

			if got := templates.IsCurrentPath(ctx, tt.linkPath); got != tt.want {
				t.Errorf("IsCurrentPath(ctx, %q) = %v, want %v", tt.linkPath, got, tt.want)
			}
		})
	}
}

// TestCurrentPathMiddleware verifies that the middleware stores the request path
// (and not the query string alongside it) so the handler downstream sees the
// path IsCurrentPath compares against.
//
// [Ja] TestCurrentPathMiddleware は、ミドルウェアがリクエストパスを (クエリ文字列を
// 添えずに) 保存し、後段のハンドラーが IsCurrentPath の比較対象となるパスを見られることを
// 検証します。
func TestCurrentPathMiddleware(t *testing.T) {
	t.Parallel()

	var isCurrent bool
	handler := templates.CurrentPathMiddleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		isCurrent = templates.IsCurrentPath(r.Context(), templates.HomePath().String())
	}))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/home?from=settings", nil))

	if !isCurrent {
		t.Errorf("IsCurrentPath(ctx, %q) = false, want true", templates.HomePath())
	}
}
