package components_test

import (
	"context"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/templates/components"
)

// TestFormErrorsはFormErrorsが各グローバルメッセージについてrole="alert" 付きの
// Basecoat destructiveアラートを描画し、グローバルエラーが無いとき (nilの
// ValidationError、フィールドエラーのみを持つもの、空のもの) は何も描画しないことを
// 検証します。FormErrorsは翻訳に触れないため、background contextで十分です。
func TestFormErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		formErrors   *model.ValidationError
		wantContains []string
		wantEmpty    bool
	}{
		{
			name:       "フォーム全体のエラーが1件",
			formErrors: &model.ValidationError{Global: []string{"メールアドレスかパスワードが正しくありません"}},
			wantContains: []string{
				`<div class="alert" data-variant="destructive" role="alert">`,
				`<h2>メールアドレスかパスワードが正しくありません</h2>`,
			},
		},
		{
			name:       "フォーム全体のエラーが複数件",
			formErrors: &model.ValidationError{Global: []string{"エラー1", "エラー2"}},
			wantContains: []string{
				`<h2>エラー1</h2>`,
				`<h2>エラー2</h2>`,
			},
		},
		{
			name:       "フィールドのエラーだけなら何も描画しない",
			formErrors: &model.ValidationError{Fields: map[string][]string{"email": {"入力してください"}}},
			wantEmpty:  true,
		},
		{
			name:       "nilなら何も描画しない",
			formErrors: nil,
			wantEmpty:  true,
		},
		{
			name:       "空なら何も描画しない",
			formErrors: model.NewValidationError(),
			wantEmpty:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var buf strings.Builder
			if err := components.FormErrors(tt.formErrors).Render(context.Background(), &buf); err != nil {
				t.Fatalf("描画に失敗: %v", err)
			}

			got := buf.String()
			if tt.wantEmpty {
				if strings.TrimSpace(got) != "" {
					t.Errorf("出力 = %q、空を期待", got)
				}
				return
			}
			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("出力に %q が含まれていない\n出力: %s", want, got)
				}
			}
		})
	}
}

// TestFieldErrorsはFieldErrorsが指定フィールドについてメッセージごとに
// role="alert" の段落を描画し、各段落が自身のid="{field}-error-{i}" を持ち (入力欄が
// aria-describedbyでその全てを参照できるように)、フィールドにエラーが無ければ何も
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
			name:       "1件のメッセージは連番のidとroleを持つ",
			field:      "email",
			formErrors: &model.ValidationError{Fields: map[string][]string{"email": {"入力してください"}}},
			wantContains: []string{
				`<p role="alert" id="email-error-0">入力してください</p>`,
			},
		},
		{
			name:  "どのメッセージもそれぞれ連番のidを持つ",
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
			name:       "他のフィールドのエラーは描画しない",
			field:      "password",
			formErrors: &model.ValidationError{Fields: map[string][]string{"email": {"入力してください"}}},
			wantEmpty:  true,
		},
		{
			name:       "nilなら何も描画しない",
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
				t.Fatalf("描画に失敗: %v", err)
			}

			got := buf.String()
			if tt.wantEmpty {
				if strings.TrimSpace(got) != "" {
					t.Errorf("出力 = %q、空を期待", got)
				}
				return
			}
			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("出力に %q が含まれていない\n出力: %s", want, got)
				}
			}
		})
	}
}

// TestFieldErrorsDescribedByはFieldErrorsDescribedByがフィールドの全エラー
// メッセージidを (順序どおり空白区切りで) 並べ、入力欄がaria-describedbyからその
// 全てを参照できることと、フィールドにエラーが無いときは "" を返すことを検証します。idは
// FieldErrorsが各 <p> に付与するものと一致していなければなりません。
func TestFieldErrorsDescribedBy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		field      string
		formErrors *model.ValidationError
		want       string
	}{
		{
			name:       "エラーが1件ならidを1つ返す",
			field:      "email",
			formErrors: &model.ValidationError{Fields: map[string][]string{"email": {"入力してください"}}},
			want:       "email-error-0",
		},
		{
			name:  "エラーが複数件なら順にスペース区切りのidを返す",
			field: "email",
			formErrors: &model.ValidationError{Fields: map[string][]string{
				"email": {"入力してください", "正しいメールアドレスを入力してください"},
			}},
			want: "email-error-0 email-error-1",
		},
		{
			name:       "そのフィールドのエラーが無ければ空文字列を返す",
			field:      "password",
			formErrors: &model.ValidationError{Fields: map[string][]string{"email": {"入力してください"}}},
			want:       "",
		},
		{
			name:       "nilなら空文字列を返す",
			field:      "email",
			formErrors: nil,
			want:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := components.FieldErrorsDescribedBy(tt.field, tt.formErrors); got != tt.want {
				t.Errorf("FieldErrorsDescribedBy(%q) = %q、期待値 = %q", tt.field, got, tt.want)
			}
		})
	}
}

// TestFormErrorSummaryは、要約がメッセージ1つにつき1つのリンクを、メッセージの
// 保持順ではなく与えられたフィールドの順に並べること、そして各リンクがそのフィールドの
// ラベルを載せて入力欄へ導くことを検証します。順序が問題になるのは、この一覧が拒否された
// フォームを訪問者が辿る道であり、mapの反復順ではその上を飛び回ることになるためです。
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
		t.Fatalf("描画に失敗: %v", err)
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
			t.Errorf("要約に %q が含まれていない\n出力: %s", want, got)
		}
	}

	if strings.Index(got, `href="#title"`) > strings.Index(got, `href="#body"`) {
		t.Errorf("要約が与えられたフィールドの順に並んでいない\n出力: %s", got)
	}
}

// TestFormErrorSummary_NothingToListは、並べる対象のフィールドがいずれもメッセージを
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
		{name: "フォーム全体のメッセージだけ", errors: globalOnly},
		{name: "一覧に無いフィールドのメッセージ", errors: otherField},
		{name: "未送信", errors: nil},
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
				t.Fatalf("描画に失敗: %v", err)
			}

			if got := buf.String(); strings.TrimSpace(got) != "" {
				t.Errorf("要約が描かれている: %q", got)
			}
		})
	}
}
