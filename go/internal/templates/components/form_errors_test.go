package components_test

import (
	"context"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/templates/components"
)

// TestFormErrors verifies that FormErrors renders a Basecoat destructive alert
// with role="alert" for each global message, and renders nothing when there are
// no global errors (a nil ValidationError, one carrying only field errors, or an
// empty one). FormErrors touches no translations, so a background context is
// enough.
//
// [Ja] TestFormErrors は FormErrors が各グローバルメッセージについて role="alert" 付きの
// Basecoat destructive アラートを描画し、グローバルエラーが無いとき (nil の
// ValidationError、フィールドエラーのみを持つもの、空のもの) は何も描画しないことを
// 検証します。FormErrors は翻訳に触れないため、background context で十分です。
func TestFormErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		formErrors   *model.ValidationError
		wantContains []string
		wantEmpty    bool
	}{
		{
			name:       "single global error",
			formErrors: &model.ValidationError{Global: []string{"メールアドレスかパスワードが正しくありません"}},
			wantContains: []string{
				`<div class="alert" data-variant="destructive" role="alert">`,
				`<h2>メールアドレスかパスワードが正しくありません</h2>`,
			},
		},
		{
			name:       "multiple global errors",
			formErrors: &model.ValidationError{Global: []string{"エラー1", "エラー2"}},
			wantContains: []string{
				`<h2>エラー1</h2>`,
				`<h2>エラー2</h2>`,
			},
		},
		{
			name:       "field errors only renders nothing",
			formErrors: &model.ValidationError{Fields: map[string][]string{"email": {"入力してください"}}},
			wantEmpty:  true,
		},
		{
			name:       "nil renders nothing",
			formErrors: nil,
			wantEmpty:  true,
		},
		{
			name:       "empty renders nothing",
			formErrors: model.NewValidationError(),
			wantEmpty:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var buf strings.Builder
			if err := components.FormErrors(tt.formErrors).Render(context.Background(), &buf); err != nil {
				t.Fatalf("render failed: %v", err)
			}

			got := buf.String()
			if tt.wantEmpty {
				if strings.TrimSpace(got) != "" {
					t.Errorf("expected no output, got %q", got)
				}
				return
			}
			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("output does not contain %q\noutput: %s", want, got)
				}
			}
		})
	}
}

// TestFieldErrors verifies that FieldErrors renders a role="alert" paragraph per
// message for the requested field, each carrying its own id="{field}-error-{i}"
// (so the control can point to every one of them with aria-describedby), and
// renders nothing when the field has no errors.
//
// [Ja] TestFieldErrors は FieldErrors が指定フィールドについてメッセージごとに
// role="alert" の段落を描画し、各段落が自身の id="{field}-error-{i}" を持ち (入力欄が
// aria-describedby でその全てを参照できるように)、フィールドにエラーが無ければ何も
// 描画しないことを検証します。
func TestFieldErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		field        string
		formErrors   *model.ValidationError
		wantContains []string
		wantEmpty    bool
	}{
		{
			name:       "single message carries the indexed id and role",
			field:      "email",
			formErrors: &model.ValidationError{Fields: map[string][]string{"email": {"入力してください"}}},
			wantContains: []string{
				`<p role="alert" id="email-error-0">入力してください</p>`,
			},
		},
		{
			name:  "every message carries its own indexed id",
			field: "email",
			formErrors: &model.ValidationError{Fields: map[string][]string{
				"email": {"入力してください", "正しいメールアドレスを入力してください"},
			}},
			wantContains: []string{
				`<p role="alert" id="email-error-0">入力してください</p>`,
				`<p role="alert" id="email-error-1">正しいメールアドレスを入力してください</p>`,
			},
		},
		{
			name:       "other field's errors are not rendered",
			field:      "password",
			formErrors: &model.ValidationError{Fields: map[string][]string{"email": {"入力してください"}}},
			wantEmpty:  true,
		},
		{
			name:       "nil renders nothing",
			field:      "email",
			formErrors: nil,
			wantEmpty:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var buf strings.Builder
			if err := components.FieldErrors(tt.field, tt.formErrors).Render(context.Background(), &buf); err != nil {
				t.Fatalf("render failed: %v", err)
			}

			got := buf.String()
			if tt.wantEmpty {
				if strings.TrimSpace(got) != "" {
					t.Errorf("expected no output, got %q", got)
				}
				return
			}
			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("output does not contain %q\noutput: %s", want, got)
				}
			}
		})
	}
}

// TestFieldErrorsDescribedBy verifies that FieldErrorsDescribedBy lists every
// error-message id for a field (space-separated, in order) so a control can
// reference all of them from aria-describedby, and returns "" when the field has
// no errors. The ids must match the ones FieldErrors stamps on each <p>.
//
// [Ja] TestFieldErrorsDescribedBy は FieldErrorsDescribedBy がフィールドの全エラー
// メッセージ id を (順序どおり空白区切りで) 並べ、入力欄が aria-describedby からその
// 全てを参照できることと、フィールドにエラーが無いときは "" を返すことを検証します。id は
// FieldErrors が各 <p> に付与するものと一致していなければなりません。
func TestFieldErrorsDescribedBy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		field      string
		formErrors *model.ValidationError
		want       string
	}{
		{
			name:       "single error yields one id",
			field:      "email",
			formErrors: &model.ValidationError{Fields: map[string][]string{"email": {"入力してください"}}},
			want:       "email-error-0",
		},
		{
			name:  "multiple errors yield space-separated ids in order",
			field: "email",
			formErrors: &model.ValidationError{Fields: map[string][]string{
				"email": {"入力してください", "正しいメールアドレスを入力してください"},
			}},
			want: "email-error-0 email-error-1",
		},
		{
			name:       "no error for the field yields empty string",
			field:      "password",
			formErrors: &model.ValidationError{Fields: map[string][]string{"email": {"入力してください"}}},
			want:       "",
		},
		{
			name:       "nil yields empty string",
			field:      "email",
			formErrors: nil,
			want:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := components.FieldErrorsDescribedBy(tt.field, tt.formErrors); got != tt.want {
				t.Errorf("FieldErrorsDescribedBy(%q) = %q, want %q", tt.field, got, tt.want)
			}
		})
	}
}
