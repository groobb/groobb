package components_test

import (
	"context"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/i18n"
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

// TestFormErrorSummary verifies that the summary lists one link per message, in
// the order the fields are given rather than the order the messages are stored
// in, and that each link carries the field's label and leads to the control. The
// order matters because the list is how a visitor walks a refused form, and a
// map's iteration order would have them jump around it.
//
// [Ja] TestFormErrorSummary は、要約がメッセージ 1 つにつき 1 つのリンクを、メッセージの
// 保持順ではなく与えられたフィールドの順に並べること、そして各リンクがそのフィールドの
// ラベルを載せて入力欄へ導くことを検証します。順序が問題になるのは、この一覧が拒否された
// フォームを訪問者が辿る道であり、map の反復順ではその上を飛び回ることになるためです。
func TestFormErrorSummary(t *testing.T) {
	t.Parallel()

	errors := model.NewValidationError()
	errors.AddField("body", "本文を入力してください")
	errors.AddField("title", "タイトルを入力してください")

	ctx := i18n.SetLocale(context.Background(), model.LocaleJa)

	var buf strings.Builder
	if err := components.FormErrorSummary(components.FormErrorSummaryData{
		HeadingKey: "thread_new_errors_heading",
		Fields: []components.FormErrorSummaryField{
			{Name: "title", LabelKey: "thread_new_title_label"},
			{Name: "body", LabelKey: "thread_new_body_label"},
		},
		Errors: errors,
	}).Render(ctx, &buf); err != nil {
		t.Fatalf("render failed: %v", err)
	}

	got := buf.String()
	for _, want := range []string{
		"<h2>入力内容を確認してください</h2>",
		`<a href="#title" class="block min-h-6 underline">`,
		"タイトル: タイトルを入力してください",
		`<a href="#body" class="block min-h-6 underline">`,
		"最初の投稿: 本文を入力してください",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("要約に %q が含まれていない\noutput: %s", want, got)
		}
	}

	if strings.Index(got, `href="#title"`) > strings.Index(got, `href="#body"`) {
		t.Errorf("要約が与えられたフィールドの順に並んでいない\noutput: %s", got)
	}
}

// TestFormErrorSummary_NothingToList verifies that the summary draws nothing when
// none of the listed fields has a message: a submission refused as a whole, a
// message about a field the form does not list, and a form that was never
// submitted. A heading over an empty list would say there is something to correct
// where there is not.
//
// [Ja] TestFormErrorSummary_NothingToList は、並べる対象のフィールドがいずれもメッセージを
// 持たないとき、要約が何も描かないことを検証します。全体として拒否された送信、フォームが
// 並べないフィールドについてのメッセージ、そして送信されていないフォームです。空の一覧に
// 載った見出しは、直すものが無いところに直すものがあると述べてしまいます。
func TestFormErrorSummary_NothingToList(t *testing.T) {
	t.Parallel()

	globalOnly := model.NewValidationError()
	globalOnly.AddGlobal("投稿を保存できませんでした")

	otherField := model.NewValidationError()
	otherField.AddField("language", "主言語を選んでください")

	tests := []struct {
		name   string
		errors *model.ValidationError
	}{
		{name: "form-wide message only", errors: globalOnly},
		{name: "message about an unlisted field", errors: otherField},
		{name: "not submitted", errors: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := i18n.SetLocale(context.Background(), model.LocaleJa)

			var buf strings.Builder
			if err := components.FormErrorSummary(components.FormErrorSummaryData{
				HeadingKey: "thread_new_errors_heading",
				Fields:     []components.FormErrorSummaryField{{Name: "title", LabelKey: "thread_new_title_label"}},
				Errors:     tt.errors,
			}).Render(ctx, &buf); err != nil {
				t.Fatalf("render failed: %v", err)
			}

			if got := buf.String(); strings.TrimSpace(got) != "" {
				t.Errorf("要約が描かれている: %q", got)
			}
		})
	}
}
