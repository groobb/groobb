package validator_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/validator"
)

// TestPostCreateValidator_Validate covers the rules a reply body follows: a
// body that says something and fits the limit is accepted, including at the
// boundary and where emoji make the code-point count differ from the byte
// count, while a body that is empty, has nothing visible in it, is over the
// limit, or cannot be stored is refused with a field error on body.
//
// [Ja] TestPostCreateValidator_Validate は返信の本文が従う規則を網羅する。何かを述べて
// おり上限に収まる本文は、境界のちょうどの長さや、絵文字によってコードポイント数と
// バイト数が食い違う場合も含めて受け付けられ、空・見える文字が1つも無い・長さ超過・
// 保存できない本文は body のフィールドエラーで拒否される。
func TestPostCreateValidator_Validate(t *testing.T) {
	t.Parallel()

	v := validator.NewPostCreateValidator()

	tests := []struct {
		name    string
		body    string
		wantErr bool
	}{
		{
			name: "正常系: 本文",
			body: "スレッドへの返信です。",
		},
		{
			name: "正常系: 境界の10,000文字",
			body: strings.Repeat("あ", validator.PostBodyMaxLength),
		},
		{
			name: "正常系: 絵文字はコードポイント数で数える",
			body: strings.Repeat("😀", validator.PostBodyMaxLength),
		},
		{
			name: "正常系: 改行を含む本文",
			body: "1行目\n\n3行目",
		},
		{
			// The count is of the normalized body, so a body at the limit is not
			// refused for the CR the browser added to each line ending.
			//
			// [Ja] 数えるのは正規化後の本文であるため、上限ちょうどの本文が、ブラウザが
			// 各行末に足したCRのせいで拒否されることはない。
			name: "正常系: CRLFをLFに正規化した後の長さで数える",
			body: strings.Repeat("あ\r\n", validator.PostBodyMaxLength/2),
		},
		{
			name:    "異常系: 空",
			body:    "",
			wantErr: true,
		},
		{
			name:    "異常系: 空白だけ",
			body:    " \t\n　",
			wantErr: true,
		},
		{
			// Characters that render as nothing survive trimming, so a body built out
			// of them alone would otherwise be taken as one that says something.
			//
			// [Ja] 何も描かない文字は空白の除去を生き延びるため、それだけでできた本文は、
			// そのままでは何かを述べているものとして受け取られてしまう。
			name:    "異常系: 目に見えない文字だけ",
			body:    "\u200b",
			wantErr: true,
		},
		{
			name:    "異常系: 目に見えない文字と空白だけ",
			body:    "\u200b \n　\u200b",
			wantErr: true,
		},
		{
			name:    "異常系: 10,001文字",
			body:    strings.Repeat("あ", validator.PostBodyMaxLength+1),
			wantErr: true,
		},
		{
			name:    "異常系: 絵文字が10,001文字",
			body:    strings.Repeat("😀", validator.PostBodyMaxLength+1),
			wantErr: true,
		},
		{
			name:    "異常系: 不正なUTF-8",
			body:    "本文\xff",
			wantErr: true,
		},
		{
			name:    "異常系: NULを含む",
			body:    "本文\x00",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := i18n.SetLocale(context.Background(), model.LocaleJa)

			body, err := v.Validate(ctx, validator.PostCreateValidatorInput{Body: tt.body})

			if !tt.wantErr {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if body == "" {
					t.Error("expected the normalized body, got an empty string")
				}
				return
			}

			ve := model.AsValidationError(err)
			if ve == nil {
				t.Fatalf("expected a ValidationError, got %v", err)
			}
			if !ve.HasFieldError("body") {
				t.Errorf("expected a field error on body, got %#v", ve.Fields)
			}
			if body != "" {
				t.Errorf("expected no body on failure, got %q", body)
			}
		})
	}
}

// TestPostCreateValidator_ValidateNormalizes verifies what a valid body is
// stored as: line endings unified to LF, and the whitespace around it left
// exactly as it was typed.
//
// [Ja] TestPostCreateValidator_ValidateNormalizes は、妥当な本文が何として保存されるか
// を検証する。改行はLFに統一され、その周りの空白は打たれたままに残る。
func TestPostCreateValidator_ValidateNormalizes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "CRLFをLFに統一する",
			body: "1行目\r\n2行目",
			want: "1行目\n2行目",
		},
		{
			name: "単独のCRもLFに統一する",
			body: "1行目\r2行目",
			want: "1行目\n2行目",
		},
		{
			name: "前後の空白と空行を保つ",
			body: "\n  字下げした本文  \n",
			want: "\n  字下げした本文  \n",
		},
	}

	v := validator.NewPostCreateValidator()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := i18n.SetLocale(context.Background(), model.LocaleJa)

			body, err := v.Validate(ctx, validator.PostCreateValidatorInput{Body: tt.body})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if body != tt.want {
				t.Errorf("body = %q, want %q", body, tt.want)
			}
		})
	}
}

// TestPostCreateValidator_ValidateVisibleText rejects invisible-only bodies
// while preserving characters that affect the rendering of visible text.
//
// [Ja] TestPostCreateValidator_ValidateVisibleText は非表示文字だけの本文を拒否し、
// 可視テキストの描画に関わる文字はそのまま保持することを検証する。
func TestPostCreateValidator_ValidateVisibleText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		text    string
		wantErr bool
	}{
		{name: "異体字セレクター1のみ", text: "\ufe00", wantErr: true},
		{name: "異体字セレクター16のみ", text: "\ufe0f", wantErr: true},
		{name: "結合書記素接合子のみ", text: "\u034f", wantErr: true},
		{name: "異体字セレクター1と空白", text: " \ufe00\t　", wantErr: true},
		{name: "異体字セレクター16と空白", text: " \ufe0f\t　", wantErr: true},
		{name: "結合書記素接合子と空白", text: " \u034f\t　", wantErr: true},
		{name: "点のない点字のマスのみ", text: "\u2800", wantErr: true},
		{name: "点のない点字のマスと空白", text: " \u2800\t　", wantErr: true},
		{name: "非表示文字の組み合わせ", text: "\ufe00\ufe0f\u034f\u200b\u2800", wantErr: true},
		{name: "異体字セレクターを含む絵文字", text: "\u2764\ufe0f"},
		{name: "結合アクセント", text: "e\u0301"},
		{name: "ZWJを含む絵文字", text: "👨‍👩‍👧‍👦"},
		{name: "異体字セレクターを含む漢字", text: "葛\U000e0100"},
		{name: "可視文字の前後に結合書記素接合子", text: "\u034fあ\u034f"},
		{name: "点のあるマスを含む点字", text: "⠓⠊\u2800⠞⠓⠑⠗⠑"},
	}

	v := validator.NewPostCreateValidator()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := i18n.SetLocale(context.Background(), model.LocaleJa)

			body, err := v.Validate(ctx, validator.PostCreateValidatorInput{Body: tt.text})
			if !tt.wantErr {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if body != tt.text {
					t.Errorf("body = %q, want %q", body, tt.text)
				}
				return
			}

			ve := model.AsValidationError(err)
			if ve == nil {
				t.Fatalf("expected a ValidationError, got %v", err)
			}
			want := []string{i18n.T(ctx, "validation_required")}
			if messages := ve.GetFieldErrors("body"); !slices.Equal(messages, want) {
				t.Errorf("body errors = %q, want %q", messages, want)
			}
			if body != "" {
				t.Errorf("expected no body on failure, got %q", body)
			}
		})
	}
}
