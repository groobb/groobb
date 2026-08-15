package validator_test

import (
	"context"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/query"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/validator"
)

// newCommunityValidator builds a CommunityCreateValidator bound to the test
// transaction so the identifier uniqueness check reads rows seeded in the same tx
// and rolls back afterwards. It also returns the repository so a test can seed a
// community.
//
// [Ja] newCommunityValidator はテスト用トランザクションに束ねた
// CommunityCreateValidator を作る。識別子の一意性チェックが同じ tx に仕込んだ行を読み、
// テスト後にロールバックされる。テストがコミュニティを仕込めるようリポジトリも返す。
func newCommunityValidator(t *testing.T) (*validator.CommunityCreateValidator, *repository.CommunityRepository) {
	t.Helper()
	db, tx := testutil.SetupTx(t)
	communityRepo := repository.NewCommunityRepository(query.New(db)).WithTx(tx)
	return validator.NewCommunityCreateValidator(communityRepo), communityRepo
}

// TestCommunityCreateValidator_Validate covers the format checks: a valid name
// and identifier (including the 30- and 20-character boundaries) succeed, while
// an empty or over-length name, and an empty, over-length, malformed, or
// reserved identifier each fail with a field error on the expected field. The
// subtests share one transaction and run serially because a pgx.Tx is not safe
// for concurrent use.
//
// [Ja] TestCommunityCreateValidator_Validate は形式チェックを網羅する。有効な名前と
// 識別子 (30 文字・20 文字の境界を含む) は成功し、空・長さ超過の名前、および空・
// 長さ超過・形式不正・予約語の識別子はいずれも期待するフィールドのフィールドエラーで
// 失敗する。サブテストは 1 つのトランザクションを共有し、pgx.Tx は並行利用が安全で
// ないため直列に実行する。
func TestCommunityCreateValidator_Validate(t *testing.T) {
	t.Parallel()

	v, _ := newCommunityValidator(t)
	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)

	tests := []struct {
		name          string
		input         validator.CommunityCreateValidatorInput
		wantErr       bool
		expectedField string
	}{
		{
			name:    "正常系: 有効な名前と識別子",
			input:   validator.CommunityCreateValidatorInput{Name: "ゲーム好きの集い", Identifier: "game-lovers"},
			wantErr: false,
		},
		{
			name:    "正常系: 境界の 30 文字の名前 (マルチバイトでもルーン数で数える)",
			input:   validator.CommunityCreateValidatorInput{Name: strings.Repeat("あ", 30), Identifier: "name-boundary"},
			wantErr: false,
		},
		{
			name:    "正常系: 境界の 20 文字の識別子",
			input:   validator.CommunityCreateValidatorInput{Name: "境界テスト", Identifier: strings.Repeat("a", 20)},
			wantErr: false,
		},
		{
			name:          "異常系: 名前が空",
			input:         validator.CommunityCreateValidatorInput{Name: "", Identifier: "empty-name"},
			wantErr:       true,
			expectedField: "name",
		},
		{
			name:          "異常系: 名前が長すぎる (31 文字)",
			input:         validator.CommunityCreateValidatorInput{Name: strings.Repeat("あ", 31), Identifier: "long-name"},
			wantErr:       true,
			expectedField: "name",
		},
		{
			name:          "異常系: 識別子が空",
			input:         validator.CommunityCreateValidatorInput{Name: "識別子が空", Identifier: ""},
			wantErr:       true,
			expectedField: "identifier",
		},
		{
			name:          "異常系: 識別子が長すぎる (21 文字)",
			input:         validator.CommunityCreateValidatorInput{Name: "識別子が長い", Identifier: strings.Repeat("a", 21)},
			wantErr:       true,
			expectedField: "identifier",
		},
		{
			name:          "異常系: 識別子に使えない文字 (アンダースコア)",
			input:         validator.CommunityCreateValidatorInput{Name: "識別子の形式", Identifier: "bad_identifier"},
			wantErr:       true,
			expectedField: "identifier",
		},
		{
			name:          "異常系: 識別子が予約語",
			input:         validator.CommunityCreateValidatorInput{Name: "予約語", Identifier: "www"},
			wantErr:       true,
			expectedField: "identifier",
		},
		{
			name:          "異常系: 予約語の判定は大文字小文字を区別しない",
			input:         validator.CommunityCreateValidatorInput{Name: "予約語 (大文字)", Identifier: "WWW"},
			wantErr:       true,
			expectedField: "identifier",
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

// TestCommunityCreateValidator_Validate_IdentifierAlreadyTaken verifies the
// uniqueness (state) check: an identifier already used by another community is
// rejected on the identifier field, and the match is case-insensitive via citext.
//
// [Ja] TestCommunityCreateValidator_Validate_IdentifierAlreadyTaken は一意性 (状態)
// チェックを検証する。他のコミュニティが既に使う識別子は identifier フィールドで弾かれ、
// 照合は citext により大文字小文字を区別しない。
func TestCommunityCreateValidator_Validate_IdentifierAlreadyTaken(t *testing.T) {
	t.Parallel()

	v, communityRepo := newCommunityValidator(t)
	ctx := i18n.SetLocale(context.Background(), i18n.LangJa)

	if _, err := communityRepo.Create(ctx, repository.CreateCommunityInput{
		Name:       "先着のコミュニティ",
		Identifier: "taken-identifier",
	}); err != nil {
		t.Fatalf("前提コミュニティの作成に失敗: %v", err)
	}

	tests := []struct {
		name       string
		identifier string
	}{
		{name: "完全一致", identifier: "taken-identifier"},
		{name: "citext により大文字小文字違いも重複扱い", identifier: "Taken-Identifier"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.Validate(ctx, validator.CommunityCreateValidatorInput{
				Name:       "後発のコミュニティ",
				Identifier: tt.identifier,
			})
			ve := model.AsValidationError(err)
			if ve == nil {
				t.Fatalf("Validate() error = %v, want *model.ValidationError", err)
			}
			errs := ve.GetFieldErrors("identifier")
			if len(errs) == 0 {
				t.Fatalf("identifier フィールドのエラーが無い: %+v", ve.Fields)
			}
			if errs[0] != "この URL 識別子は既に使用されています" {
				t.Errorf("identifier エラー = %q, want %q", errs[0], "この URL 識別子は既に使用されています")
			}
		})
	}
}

// TestCommunityCreateValidator_Validate_Messages verifies the name and identifier
// error messages are localized for each supported locale, and that a reserved
// identifier is reported with its own message rather than the format one.
//
// [Ja] TestCommunityCreateValidator_Validate_Messages は名前と識別子のエラーメッセージが
// サポートする各ロケールでローカライズされること、および予約語の識別子が形式エラーでは
// なく専用のメッセージで報告されることを検証する。
func TestCommunityCreateValidator_Validate_Messages(t *testing.T) {
	t.Parallel()

	v, _ := newCommunityValidator(t)

	tests := []struct {
		name          string
		locale        string
		input         validator.CommunityCreateValidatorInput
		expectedField string
		wantMsg       string
	}{
		{
			name:          "ja: 名前が長すぎる",
			locale:        i18n.LangJa,
			input:         validator.CommunityCreateValidatorInput{Name: strings.Repeat("あ", 31), Identifier: "msg-ja-name"},
			expectedField: "name",
			wantMsg:       "コミュニティ名は 30 文字以内で入力してください",
		},
		{
			name:          "en: 名前が長すぎる",
			locale:        i18n.LangEn,
			input:         validator.CommunityCreateValidatorInput{Name: strings.Repeat("a", 31), Identifier: "msg-en-name"},
			expectedField: "name",
			wantMsg:       "must be at most 30 characters",
		},
		{
			name:          "ja: 識別子が長すぎる",
			locale:        i18n.LangJa,
			input:         validator.CommunityCreateValidatorInput{Name: "メッセージ", Identifier: strings.Repeat("a", 21)},
			expectedField: "identifier",
			wantMsg:       "URL 識別子は 20 文字以内で入力してください",
		},
		{
			name:          "en: 識別子が長すぎる",
			locale:        i18n.LangEn,
			input:         validator.CommunityCreateValidatorInput{Name: "メッセージ", Identifier: strings.Repeat("a", 21)},
			expectedField: "identifier",
			wantMsg:       "must be at most 20 characters",
		},
		{
			name:          "ja: 識別子の形式が不正",
			locale:        i18n.LangJa,
			input:         validator.CommunityCreateValidatorInput{Name: "メッセージ", Identifier: "bad_identifier"},
			expectedField: "identifier",
			wantMsg:       "URL 識別子は半角英数字とハイフンのみ使用できます",
		},
		{
			name:          "en: 識別子の形式が不正",
			locale:        i18n.LangEn,
			input:         validator.CommunityCreateValidatorInput{Name: "メッセージ", Identifier: "bad_identifier"},
			expectedField: "identifier",
			wantMsg:       "may only contain letters, numbers, and hyphens",
		},
		{
			name:          "ja: 識別子が予約語",
			locale:        i18n.LangJa,
			input:         validator.CommunityCreateValidatorInput{Name: "メッセージ", Identifier: "www"},
			expectedField: "identifier",
			wantMsg:       "この URL 識別子は使用できません",
		},
		{
			name:          "en: 識別子が予約語",
			locale:        i18n.LangEn,
			input:         validator.CommunityCreateValidatorInput{Name: "メッセージ", Identifier: "www"},
			expectedField: "identifier",
			wantMsg:       "is reserved and cannot be used",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := i18n.SetLocale(context.Background(), tt.locale)

			err := v.Validate(ctx, tt.input)

			ve := model.AsValidationError(err)
			if ve == nil {
				t.Fatalf("Validate() error = %v, want *model.ValidationError", err)
			}
			errs := ve.GetFieldErrors(tt.expectedField)
			if len(errs) == 0 {
				t.Fatalf("%q フィールドのエラーが無い: %+v", tt.expectedField, ve.Fields)
			}
			if errs[0] != tt.wantMsg {
				t.Errorf("%q エラー = %q, want %q", tt.expectedField, errs[0], tt.wantMsg)
			}
		})
	}
}
