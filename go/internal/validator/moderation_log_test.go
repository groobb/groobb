package validator_test

import (
	"context"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/validator"
)

// TestModerationLogCreateValidator_Validateは、モデレーションの操作に記録する理由が
// 従う規則を網羅する。上限に収まる注記は受け付けられ、書かれなかった注記も書かれた注記と
// 同じく受け付けられる。上限を超えた注記と、アプリケーションが保存できない注記は、reasonの
// フィールドエラーで拒否される。
func TestModerationLogCreateValidator_Validate(t *testing.T) {
	t.Parallel()

	v := validator.NewModerationLogCreateValidator()

	tests := []struct {
		name    string
		reason  string
		want    string
		wantErr bool
	}{
		{
			name:   "正常系: 理由",
			reason: "規約に反する書き込みが続いたため。",
			want:   "規約に反する書き込みが続いたため。",
		},
		{
			name:   "正常系: 空",
			reason: "",
			want:   "",
		},
		{
			name:   "正常系: 空白だけは空として記録する",
			reason: " \t\n　",
			want:   "",
		},
		{
			name:   "正常系: 前後の空白を取り除く",
			reason: "\n  理由  \n",
			want:   "理由",
		},
		{
			name:   "正常系: 改行を含む理由",
			reason: "1行目\r\n2行目",
			want:   "1行目\n2行目",
		},
		{
			name:   "正常系: 境界の500文字",
			reason: strings.Repeat("あ", validator.ModerationReasonMaxLength),
			want:   strings.Repeat("あ", validator.ModerationReasonMaxLength),
		},
		{
			// 数えるのは前後の空白を除いて正規化した後の理由であるため、上限ちょうどの
			// 注記が、ブラウザが各行末に足したCRのせいで拒否されることはない。
			name:   "正常系: 正規化した後の長さで数える",
			reason: " " + strings.Repeat("あ\r\n", validator.ModerationReasonMaxLength/2),
			want:   strings.TrimSpace(strings.Repeat("あ\n", validator.ModerationReasonMaxLength/2)),
		},
		{
			name:   "正常系: 絵文字はコードポイント数で数える",
			reason: strings.Repeat("😀", validator.ModerationReasonMaxLength),
			want:   strings.Repeat("😀", validator.ModerationReasonMaxLength),
		},
		{
			name:    "異常系: 501文字",
			reason:  strings.Repeat("あ", validator.ModerationReasonMaxLength+1),
			wantErr: true,
		},
		{
			name:    "異常系: 絵文字が501文字",
			reason:  strings.Repeat("😀", validator.ModerationReasonMaxLength+1),
			wantErr: true,
		},
		{
			name:    "異常系: 不正なUTF-8",
			reason:  "理由\xff",
			wantErr: true,
		},
		{
			name:    "異常系: NULを含む",
			reason:  "理由\x00",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := i18n.SetLocale(context.Background(), model.LocaleJa)

			reason, err := v.Validate(ctx, validator.ModerationLogCreateValidatorInput{Reason: tt.reason})

			if !tt.wantErr {
				if err != nil {
					t.Fatalf("予期しないエラー: %v", err)
				}
				if reason != tt.want {
					t.Errorf("Validate() = %q、期待値 = %q", reason, tt.want)
				}
				return
			}

			ve := model.AsValidationError(err)
			if ve == nil {
				t.Fatalf("エラー = %v、期待値 = ValidationError", err)
			}
			if !ve.HasFieldError("reason") {
				t.Errorf("reasonフィールドのエラーが無い: %#v", ve.Fields)
			}
			if reason != "" {
				t.Errorf("失敗時のreason = %q、期待値は空文字列", reason)
			}
		})
	}
}
