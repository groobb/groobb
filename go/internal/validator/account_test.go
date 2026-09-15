package validator_test

import (
	"context"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/validator"
)

// newAccountValidatorはテスト専用のデータベース上にAccountCreateValidatorを
// 作り、atnameの一意性チェックがそこへ仕込んだ行を読むようにする。
func newAccountValidator(t *testing.T, db *database.DB) *validator.AccountCreateValidator {
	t.Helper()
	userRepo := repository.NewUserRepository(db)
	return validator.NewAccountCreateValidator(userRepo)
}

// TestAccountCreateValidator_Validateは形式チェック (atnameの形とパスワード
// ポリシー) を網羅する。有効なatname (20文字の境界を含む) と有効で一致するパスワードは
// 成功し、空・長さ超過・形式不正のatname、および空・短すぎ・長すぎ・不一致のパスワードは
// いずれも期待するフィールドのフィールドエラーで失敗する。
func TestAccountCreateValidator_Validate(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	v := newAccountValidator(t, db)
	ctx := i18n.SetLocale(context.Background(), model.LocaleJa)

	tests := []struct {
		name          string
		input         validator.AccountCreateValidatorInput
		wantErr       bool
		expectedField string
	}{
		{
			name:    "正常系: 有効なatname・パスワード・一致する確認",
			input:   validator.AccountCreateValidatorInput{Atname: "valid_user1", Password: "password123", PasswordConfirmation: "password123"},
			wantErr: false,
		},
		{
			name:    "正常系: 境界の20文字atname",
			input:   validator.AccountCreateValidatorInput{Atname: strings.Repeat("a", 20), Password: "password123", PasswordConfirmation: "password123"},
			wantErr: false,
		},
		{
			name:          "異常系: atnameが空",
			input:         validator.AccountCreateValidatorInput{Atname: "", Password: "password123", PasswordConfirmation: "password123"},
			wantErr:       true,
			expectedField: "atname",
		},
		{
			name:          "異常系: atnameが長すぎる (21文字)",
			input:         validator.AccountCreateValidatorInput{Atname: strings.Repeat("a", 21), Password: "password123", PasswordConfirmation: "password123"},
			wantErr:       true,
			expectedField: "atname",
		},
		{
			name:          "異常系: atnameに使えない文字",
			input:         validator.AccountCreateValidatorInput{Atname: "bad-name!", Password: "password123", PasswordConfirmation: "password123"},
			wantErr:       true,
			expectedField: "atname",
		},
		// 退会の墓標atname ("deleted-<id>") は、ハイフン単体がここで拒否される
		// ことに依存する。フォームがこれを受け付けるようになると、退会がatnameを上書き
		// する値をアカウントが保持でき、users.atnameのUNIQUE制約でその退会を止められる。
		// 上のケースはハイフンと "!" を同時に含むため、ハイフンが許可されても失敗し続ける。
		{
			name:          "異常系: atnameのハイフン (退会の墓標atnameを到達不能に保つ)",
			input:         validator.AccountCreateValidatorInput{Atname: "deleted-1", Password: "password123", PasswordConfirmation: "password123"},
			wantErr:       true,
			expectedField: "atname",
		},
		{
			name:          "異常系: パスワードが空",
			input:         validator.AccountCreateValidatorInput{Atname: "valid_user2", Password: "", PasswordConfirmation: ""},
			wantErr:       true,
			expectedField: "password",
		},
		{
			name:          "異常系: パスワードが短すぎる",
			input:         validator.AccountCreateValidatorInput{Atname: "valid_user3", Password: "short", PasswordConfirmation: "short"},
			wantErr:       true,
			expectedField: "password",
		},
		{
			name:          "異常系: パスワードが長すぎる",
			input:         validator.AccountCreateValidatorInput{Atname: "valid_user4", Password: strings.Repeat("a", 73), PasswordConfirmation: strings.Repeat("a", 73)},
			wantErr:       true,
			expectedField: "password",
		},
		{
			name:          "異常系: 確認が空",
			input:         validator.AccountCreateValidatorInput{Atname: "valid_user5", Password: "password123", PasswordConfirmation: ""},
			wantErr:       true,
			expectedField: "password_confirmation",
		},
		{
			name:          "異常系: 確認が一致しない",
			input:         validator.AccountCreateValidatorInput{Atname: "valid_user6", Password: "password123", PasswordConfirmation: "password456"},
			wantErr:       true,
			expectedField: "password_confirmation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.Validate(ctx, tt.input)

			if !tt.wantErr {
				if err != nil {
					t.Fatalf("Validate()のエラー = %v、期待値 = nil", err)
				}
				return
			}

			ve := model.AsValidationError(err)
			if ve == nil {
				t.Fatalf("Validate()のエラー = %v、期待値 = *model.ValidationError", err)
			}
			if !ve.HasFieldError(tt.expectedField) {
				t.Errorf("%q フィールドのエラーが無い: %+v", tt.expectedField, ve.Fields)
			}
		})
	}
}

// TestAccountCreateValidator_Validate_AtnameAlreadyTakenは一意性 (状態)
// チェックを検証する。他ユーザーが既に使うatnameはatnameフィールドで弾かれ、照合は
// NOCASE照合により大文字小文字を区別しない。
func TestAccountCreateValidator_Validate_AtnameAlreadyTaken(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)

	v := newAccountValidator(t, db)
	ctx := i18n.SetLocale(context.Background(), model.LocaleJa)

	testutil.NewUserBuilder(t, db).WithAtname("takenname").Build()

	tests := []struct {
		name   string
		atname string
	}{
		{name: "完全一致", atname: "takenname"},
		{name: "NOCASE照合により大文字小文字違いも重複扱い", atname: "TakenName"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.Validate(ctx, validator.AccountCreateValidatorInput{
				Atname:               tt.atname,
				Password:             "password123",
				PasswordConfirmation: "password123",
			})
			ve := model.AsValidationError(err)
			if ve == nil {
				t.Fatalf("Validate()のエラー = %v、期待値 = *model.ValidationError", err)
			}
			errs := ve.GetFieldErrors("atname")
			if len(errs) == 0 {
				t.Fatalf("atnameフィールドのエラーが無い: %+v", ve.Fields)
			}
			if errs[0] != "このアットネームは既に使用されています" {
				t.Errorf("atnameエラー = %q、期待値 = %q", errs[0], "このアットネームは既に使用されています")
			}
		})
	}
}

// TestAccountCreateValidator_Validate_AtnameMessagesはatnameの形式エラー
// メッセージがサポートする各ロケールでローカライズされることを検証する。
func TestAccountCreateValidator_Validate_AtnameMessages(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	v := newAccountValidator(t, db)

	tests := []struct {
		name    string
		locale  model.Locale
		atname  string
		wantMsg string
	}{
		{name: "ja: 長すぎる", locale: model.LocaleJa, atname: strings.Repeat("a", 21), wantMsg: "アットネームは20文字以内で入力してください"},
		{name: "en: 長すぎる", locale: model.LocaleEn, atname: strings.Repeat("a", 21), wantMsg: "must be at most 20 characters"},
		{name: "ja: 不正な文字", locale: model.LocaleJa, atname: "bad-name!", wantMsg: "アットネームは半角英数字とアンダースコアのみ使用できます"},
		{name: "en: 不正な文字", locale: model.LocaleEn, atname: "bad-name!", wantMsg: "may only contain letters, numbers, and underscores"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := i18n.SetLocale(context.Background(), tt.locale)
			err := v.Validate(ctx, validator.AccountCreateValidatorInput{
				Atname:               tt.atname,
				Password:             "password123",
				PasswordConfirmation: "password123",
			})
			ve := model.AsValidationError(err)
			if ve == nil {
				t.Fatalf("Validate()のエラー = %v、期待値 = *model.ValidationError", err)
			}
			errs := ve.GetFieldErrors("atname")
			if len(errs) == 0 {
				t.Fatalf("atnameフィールドのエラーが無い: %+v", ve.Fields)
			}
			if errs[0] != tt.wantMsg {
				t.Errorf("atnameエラー = %q、期待値 = %q", errs[0], tt.wantMsg)
			}
		})
	}
}

// TestIsValidAtnameは、フォームの外の呼び出し元が尋ねる規則を検証する。
// atnameが使えるのは、長さの上限に収まり、許可された文字だけを持つときである。
func TestIsValidAtname(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		atname string
		want   bool
	}{
		{name: "英字・数字・アンダースコア", atname: "seed_user1", want: true},
		{name: "長さの上限ちょうど", atname: strings.Repeat("a", validator.AtnameMaxLength), want: true},
		{name: "長さの上限を超える", atname: strings.Repeat("a", validator.AtnameMaxLength+1), want: false},
		{name: "空", atname: "", want: false},
		{name: "空白を含む", atname: "seed user", want: false},
		{name: "ハイフンを含む", atname: "seed-user", want: false},
		{name: "ASCII以外の文字を含む", atname: "シードユーザー", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := validator.IsValidAtname(tt.atname); got != tt.want {
				t.Errorf("IsValidAtname(%q) = %v、期待値 = %v", tt.atname, got, tt.want)
			}
		})
	}
}
