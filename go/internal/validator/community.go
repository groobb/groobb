package validator

import (
	"context"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
)

// CommunityNameMaxLength is the maximum length of a community name, in
// characters. The name is the human-facing display name and may be written in
// any script, so it is counted in runes rather than bytes.
//
// [Ja] CommunityNameMaxLength はコミュニティ名の最大文字数。名前は対人向けの表示名で
// あらゆる文字体系で書かれうるため、バイトではなくルーン数で数える。
const CommunityNameMaxLength = 30

// CommunityIdentifierMaxLength is the maximum length of a community identifier,
// in characters. The form's maxlength mirrors this constant.
//
// [Ja] CommunityIdentifierMaxLength はコミュニティの URL 識別子の最大文字数。フォームの
// maxlength はこの定数をミラーする。
const CommunityIdentifierMaxLength = 20

// communityIdentifierRegex is the allowed identifier format: one or more ASCII
// letters, digits, or hyphens. It requires at least one character, so an empty
// identifier fails it; the empty case is reported as "required" before the
// format check runs so the two errors do not stack. The form's pattern mirrors
// this expression.
//
// [Ja] communityIdentifierRegex は許可する URL 識別子の形式: 1 文字以上の ASCII 英数字
// またはハイフン。1 文字以上を要求するため空の識別子は不適合になるが、空のケースは
// 形式チェックの前に「必須」として報告し 2 つのエラーが重ならないようにする。フォームの
// pattern はこの式をミラーする。
var communityIdentifierRegex = regexp.MustCompile(`^[A-Za-z0-9-]+$`)

// communityIdentifierReservedWords holds the identifiers a community may not
// claim, in lower case (the lookup lower-cases its input, matching the
// case-insensitive uniqueness the citext column gives the identifier).
//
// www is reserved to leave the subdomain available. Community pages live under
// the short path /c/{identifier} while the collection routes live under
// /communities, so no identifier can mask a static route and the list needs no
// entry for one.
//
// [Ja] communityIdentifierReservedWords はコミュニティが取得できない識別子を小文字で
// 保持する (引く側で入力を小文字化しており、citext 列が識別子に与える大文字小文字を
// 区別しない一意性と揃う)。
//
// www はサブドメインでの利用余地を残すために予約する。コミュニティの画面は短縮パス
// /c/{identifier} に、コレクションのルートは /communities に置いているため、識別子が
// 静的ルートを masked することはなく、そのための項目は不要。
var communityIdentifierReservedWords = map[string]struct{}{
	"www": {},
}

// CommunityCreateValidator validates the community-creation form: the name is
// present and short enough, and the chosen identifier is well-formed, not
// reserved, and not already taken. It depends on communityRepo for the
// identifier uniqueness check (a state check against the database).
//
// [Ja] CommunityCreateValidator はコミュニティ作成フォームを検証します。名前が入力され
// 長すぎず、選んだ URL 識別子の形式が正しく、予約語でなく、既に使われていないことです。
// 識別子の一意性チェック (DB に対する状態チェック) のため communityRepo に依存します。
type CommunityCreateValidator struct {
	communityRepo *repository.CommunityRepository
}

// NewCommunityCreateValidator creates a CommunityCreateValidator.
//
// [Ja] NewCommunityCreateValidator は CommunityCreateValidator を生成します。
func NewCommunityCreateValidator(communityRepo *repository.CommunityRepository) *CommunityCreateValidator {
	return &CommunityCreateValidator{communityRepo: communityRepo}
}

// CommunityCreateValidatorInput is the input to
// CommunityCreateValidator.Validate.
//
// [Ja] CommunityCreateValidatorInput は CommunityCreateValidator.Validate の入力です。
type CommunityCreateValidatorInput struct {
	Name       string
	Identifier string
}

// Validate checks the name and the identifier, returning a
// *model.ValidationError when either is invalid, or a plain error on a genuine
// system failure (e.g. the database is unreachable). Format checks run first;
// only when they all pass is the identifier uniqueness check made against the
// database, so a malformed identifier never hits the database. An empty value
// reports a required error and skips the remaining checks on that field so the
// errors do not stack.
//
// [Ja] Validate は名前と URL 識別子を検証し、いずれかが不正なら *model.ValidationError
// を、本物のシステム障害 (例: データベースに到達できない) では素の error を返します。
// 形式チェックを先に行い、それらがすべて通ったときだけ識別子の一意性チェックを DB に
// 対して行うため、不正な識別子が DB に到達することはありません。空の値は必須エラーを
// 報告しそのフィールドの残りのチェックをスキップして、エラーが重ならないようにします。
func (v *CommunityCreateValidator) Validate(ctx context.Context, input CommunityCreateValidatorInput) error {
	ve := model.NewValidationError()

	if input.Name == "" {
		ve.AddField("name", i18n.T(ctx, "validation_required"))
	} else if utf8.RuneCountInString(input.Name) > CommunityNameMaxLength {
		ve.AddField("name", i18n.T(ctx, "validation_community_name_too_long"))
	}

	if input.Identifier == "" {
		ve.AddField("identifier", i18n.T(ctx, "validation_required"))
	} else {
		if utf8.RuneCountInString(input.Identifier) > CommunityIdentifierMaxLength {
			ve.AddField("identifier", i18n.T(ctx, "validation_community_identifier_too_long"))
		}
		if !communityIdentifierRegex.MatchString(input.Identifier) {
			ve.AddField("identifier", i18n.T(ctx, "validation_community_identifier_invalid_format"))
		}
		if _, reserved := communityIdentifierReservedWords[strings.ToLower(input.Identifier)]; reserved {
			ve.AddField("identifier", i18n.T(ctx, "validation_community_identifier_reserved"))
		}
	}

	if ve.HasErrors() {
		return ve
	}

	// State check: the identifier has passed the format checks, so it is safe to
	// look it up. A duplicate is reported explicitly; the DB UNIQUE constraint
	// remains the last line of defense for the rare check-then-insert race. The
	// lookup is case-insensitive because communities.identifier is citext.
	//
	// [Ja] 状態チェック: 識別子は形式チェックを通過済みのため引いてよい。重複は明示的に
	// 報告する。稀な「チェックしてから挿入」の競合に対しては DB の UNIQUE 制約が最終
	// 防衛線として残る。communities.identifier は citext のため照合は大文字小文字を
	// 区別しない。
	existingCommunity, err := v.communityRepo.FindByIdentifier(ctx, input.Identifier)
	if err != nil {
		return err
	}
	if existingCommunity != nil {
		ve.AddField("identifier", i18n.T(ctx, "validation_community_identifier_already_taken"))
		return ve
	}

	return nil
}
