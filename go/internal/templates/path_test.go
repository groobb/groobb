package templates_test

import (
	"testing"

	"github.com/groobb/groobb/go/internal/templates"
)

// TestStaticPaths verifies that the no-argument path helpers return the exact
// route strings registered in cmd/server/main.go, so the two never drift apart.
//
// [Ja] TestStaticPaths は引数なしのパスヘルパーが cmd/server/main.go で登録された
// ルート文字列と完全に一致することを検証し、両者が乖離しないようにします。
func TestStaticPaths(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		got  templates.Path
		want templates.Path
	}{
		{name: "SignUpPath", got: templates.SignUpPath(), want: "/sign_up"},
		{name: "SignInPath", got: templates.SignInPath(), want: "/sign_in"},
		{name: "HomePath", got: templates.HomePath(), want: "/home"},
		{name: "UserSessionPath", got: templates.UserSessionPath(), want: "/user_session"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.got != tt.want {
				t.Errorf("%s = %q, want %q", tt.name, tt.got, tt.want)
			}
		})
	}
}
