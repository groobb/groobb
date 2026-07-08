package validator_test

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/query"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/validator"
)

// newAccountValidator builds an AccountCreateValidator bound to the test
// transaction so the atname uniqueness check reads rows seeded in the same tx and
// rolls back afterwards. It also returns the tx so a test can seed a user.
//
// [Ja] newAccountValidator はテスト用トランザクションに束ねた AccountCreateValidator を
// 作る。atname の一意性チェックが同じ tx に仕込んだ行を読み、テスト後にロールバックされる。
// テストがユーザーを仕込めるよう tx も返す。
func newAccountValidator(t *testing.T) (*validator.AccountCreateValidator, pgx.Tx) {
	t.Helper()
	db, tx := testutil.SetupTx(t)
	userRepo := repository.NewUserRepository(query.New(db)).WithTx(tx)
	return validator.NewAccountCreateValidator(userRepo), tx
}

// TestAccountCreateValidator_Validate covers the format checks (atname shape and
// password policy): a valid atname (including the 20-char boundary) with a valid,
// matching password succeeds, while an empty, over-length, or malformed atname,
// and an empty, too-short, too-long, or mismatched password each fail with a
// field error on the expected field. The subtests share one transaction and run
// serially because a pgx.Tx is not safe for concurrent use.
//
// [Ja] TestAccountCreateValidator_Validate は形式チェック (atname の形とパスワード
// ポリシー) を網羅する。有効な atname (20 文字の境界を含む) と有効で一致するパスワードは
// 成功し、空・長さ超過・形式不正の atname、および空・短すぎ・長すぎ・不一致のパスワードは
// いずれも期待するフィールドのフィールドエラーで失敗する。サブテストは 1 つの
// トランザクションを共有し、pgx.Tx は並行利用が安全でないため直列に実行する。
func TestAccountCreateValidator_Validate(t *testing.T) {
	t.Parallel()

	v, _ := newAccountValidator(t)
	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)

	tests := []struct {
		name          string
		input         validator.AccountCreateValidatorInput
		wantErr       bool
		expectedField string
	}{
		{
			name:    "正常系: 有効な atname・パスワード・一致する確認",
			input:   validator.AccountCreateValidatorInput{Atname: "valid_user1", Password: "password123", PasswordConfirmation: "password123"},
			wantErr: false,
		},
		{
			name:    "正常系: 境界の 20 文字 atname",
			input:   validator.AccountCreateValidatorInput{Atname: strings.Repeat("a", 20), Password: "password123", PasswordConfirmation: "password123"},
			wantErr: false,
		},
		{
			name:          "異常系: atname が空",
			input:         validator.AccountCreateValidatorInput{Atname: "", Password: "password123", PasswordConfirmation: "password123"},
			wantErr:       true,
			expectedField: "atname",
		},
		{
			name:          "異常系: atname が長すぎる (21 文字)",
			input:         validator.AccountCreateValidatorInput{Atname: strings.Repeat("a", 21), Password: "password123", PasswordConfirmation: "password123"},
			wantErr:       true,
			expectedField: "atname",
		},
		{
			name:          "異常系: atname に使えない文字",
			input:         validator.AccountCreateValidatorInput{Atname: "bad-name!", Password: "password123", PasswordConfirmation: "password123"},
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
					t.Fatalf("Validate() error = %v, want nil", err)
				}
				return
			}

			ve := model.AsValidationError(err)
			if ve == nil {
				t.Fatalf("Validate() error = %v, want *model.ValidationError", err)
			}
			if !ve.HasFieldError(tt.expectedField) {
				t.Errorf("%q フィールドのエラーが無い: %+v", tt.expectedField, ve.Fields)
			}
		})
	}
}

// TestAccountCreateValidator_Validate_AtnameAlreadyTaken verifies the uniqueness
// (state) check: an atname already used by another user is rejected on the atname
// field, and the match is case-insensitive via citext.
//
// [Ja] TestAccountCreateValidator_Validate_AtnameAlreadyTaken は一意性 (状態)
// チェックを検証する。他ユーザーが既に使う atname は atname フィールドで弾かれ、照合は
// citext により大文字小文字を区別しない。
func TestAccountCreateValidator_Validate_AtnameAlreadyTaken(t *testing.T) {
	t.Parallel()

	v, tx := newAccountValidator(t)
	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)

	testutil.NewUserBuilder(t, tx).WithAtname("takenname").Build()

	tests := []struct {
		name   string
		atname string
	}{
		{name: "完全一致", atname: "takenname"},
		{name: "citext により大文字小文字違いも重複扱い", atname: "TakenName"},
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
				t.Fatalf("Validate() error = %v, want *model.ValidationError", err)
			}
			errs := ve.GetFieldErrors("atname")
			if len(errs) == 0 {
				t.Fatalf("atname フィールドのエラーが無い: %+v", ve.Fields)
			}
			if errs[0] != "このアットネームは既に使用されています" {
				t.Errorf("atname エラー = %q, want %q", errs[0], "このアットネームは既に使用されています")
			}
		})
	}
}

// TestAccountCreateValidator_Validate_AtnameMessages verifies the atname
// format-error messages are localized for each supported locale.
//
// [Ja] TestAccountCreateValidator_Validate_AtnameMessages は atname の形式エラー
// メッセージがサポートする各ロケールでローカライズされることを検証する。
func TestAccountCreateValidator_Validate_AtnameMessages(t *testing.T) {
	t.Parallel()

	v, _ := newAccountValidator(t)

	tests := []struct {
		name    string
		locale  string
		atname  string
		wantMsg string
	}{
		{name: "ja: 長すぎる", locale: i18n.LangJa, atname: strings.Repeat("a", 21), wantMsg: "アットネームは 20 文字以内で入力してください"},
		{name: "en: 長すぎる", locale: i18n.LangEn, atname: strings.Repeat("a", 21), wantMsg: "must be at most 20 characters"},
		{name: "ja: 不正な文字", locale: i18n.LangJa, atname: "bad-name!", wantMsg: "アットネームは半角英数字とアンダースコアのみ使用できます"},
		{name: "en: 不正な文字", locale: i18n.LangEn, atname: "bad-name!", wantMsg: "may only contain letters, numbers, and underscores"},
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
				t.Fatalf("Validate() error = %v, want *model.ValidationError", err)
			}
			errs := ve.GetFieldErrors("atname")
			if len(errs) == 0 {
				t.Fatalf("atname フィールドのエラーが無い: %+v", ve.Fields)
			}
			if errs[0] != tt.wantMsg {
				t.Errorf("atname エラー = %q, want %q", errs[0], tt.wantMsg)
			}
		})
	}
}
