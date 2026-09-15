package templates_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/groobb/groobb/go/internal/templates"
)

// TestIsCurrentPathはIsCurrentPathが今描画しているページだけに一致し、contextが
// パスを持たないときはすべてを一致とせずfalseを返すことを検証します。これによりリクエスト
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
		{name: "現在のページに一致する", setPath: true, current: "/home", linkPath: "/home", want: true},
		{name: "別のページには一致しない", setPath: true, current: "/settings", linkPath: "/home", want: false},
		{name: "配下のページには一致しない", setPath: true, current: "/settings/email/edit", linkPath: "/settings", want: false},
		{name: "contextにパスが無い", setPath: false, linkPath: "/home", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			if tt.setPath {
				ctx = templates.SetCurrentPath(ctx, tt.current)
			}

			if got := templates.IsCurrentPath(ctx, tt.linkPath); got != tt.want {
				t.Errorf("IsCurrentPath(ctx, %q) = %v、期待値 = %v", tt.linkPath, got, tt.want)
			}
		})
	}
}

// TestCurrentPathMiddlewareは、ミドルウェアがリクエストパスを (クエリ文字列を
// 添えずに) 保存し、後段のハンドラーがIsCurrentPathの比較対象となるパスを見られることを
// 検証します。
func TestCurrentPathMiddleware(t *testing.T) {
	t.Parallel()

	var isCurrent bool
	handler := templates.CurrentPathMiddleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		isCurrent = templates.IsCurrentPath(r.Context(), templates.HomePath().String())
	}))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/home?from=settings", nil))

	if !isCurrent {
		t.Errorf("IsCurrentPath(ctx, %q) = false、期待値 = true", templates.HomePath())
	}
}
