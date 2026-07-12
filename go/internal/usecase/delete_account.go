package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/validator"
)

// DeleteAccountUsecase orchestrates a user's self-service account withdrawal: it
// re-checks the current password, then in one transaction soft-deletes the user
// (stamping deleted_at), anonymizes the freed email/atname, and deletes all of the
// user's sessions. The heavier physical delete of the row and its cascading
// children is left to a later periodic purge job; this step just makes the account
// inert immediately and releases the unique identifiers.
//
// [Ja] DeleteAccountUsecase はユーザー自身によるアカウント退会を統括します。現在の
// パスワードを再確認し、1 トランザクションでユーザーを論理削除し (deleted_at を打つ)、
// 解放された email / atname を匿名化し、そのユーザーの全セッションを削除します。行と
// その CASCADE する子データのより重い物理削除は後続の定期パージジョブに委ねます。本
// ステップはアカウントを即座に無効化し、一意な識別子を解放するだけです。
type DeleteAccountUsecase struct {
	db              *pgxpool.Pool
	validator       *validator.SettingsWithdrawalDeleteValidator
	userRepo        *repository.UserRepository
	userSessionRepo *repository.UserSessionRepository
}

// NewDeleteAccountUsecase builds a DeleteAccountUsecase from the pool, its
// validator, and the repositories it persists through.
//
// [Ja] NewDeleteAccountUsecase はプール・validator・永続化に使うリポジトリから
// DeleteAccountUsecase を構築します。
func NewDeleteAccountUsecase(
	db *pgxpool.Pool,
	validator *validator.SettingsWithdrawalDeleteValidator,
	userRepo *repository.UserRepository,
	userSessionRepo *repository.UserSessionRepository,
) *DeleteAccountUsecase {
	return &DeleteAccountUsecase{
		db:              db,
		validator:       validator,
		userRepo:        userRepo,
		userSessionRepo: userSessionRepo,
	}
}

// DeleteAccountInput is the input to Execute. UserID is the signed-in user
// requesting withdrawal; CurrentPassword is the submitted form value used to
// re-authenticate the request.
//
// [Ja] DeleteAccountInput は Execute の入力です。UserID は退会を申請するサインイン済み
// ユーザー、CurrentPassword は申請を再認証するために送信されたフォーム値です。
type DeleteAccountInput struct {
	UserID          model.UserID
	CurrentPassword string
}

// Execute validates the current password and then withdraws the account.
// Validation runs first, so a wrong or missing current password returns a
// *model.ValidationError without touching any row. The anonymized email and atname
// are computed before the transaction (they are pure functions of the user id),
// keeping the transaction to persistence only.
//
// [Ja] Execute は現在のパスワードを検証してからアカウントを退会させます。バリデーションを
// 先に走らせるため、誤った / 未入力の現在のパスワードでは行に触れず
// *model.ValidationError を返します。匿名化した email と atname はトランザクションの前に
// 計算し (ユーザー id の純粋な関数のため)、トランザクションを永続化のみに保ちます。
func (uc *DeleteAccountUsecase) Execute(ctx context.Context, input DeleteAccountInput) error {
	if err := uc.validator.Validate(ctx, validator.SettingsWithdrawalDeleteValidatorInput{
		UserID:          input.UserID,
		CurrentPassword: input.CurrentPassword,
	}); err != nil {
		return err
	}

	return uc.deleteAccount(ctx, input.UserID, anonymizedEmail(input.UserID), anonymizedAtname(input.UserID))
}

// deleteAccount soft-deletes and anonymizes the user and deletes all of their
// sessions in one transaction, so the account is never left half-withdrawn: either
// both the users row is updated and the sessions are gone, or neither is. The two
// persistence steps are why this is split out of Execute (which stays pure
// orchestration).
//
// [Ja] deleteAccount はユーザーの論理削除・匿名化と全セッションの削除を 1 トランザクション
// で行い、アカウントが中途半端に退会した状態を残さないようにします (users 行の更新と
// セッションの消去が両方成るか、どちらも成らないか)。この 2 つの永続化ステップがあるため、
// 本処理を Execute (純粋なオーケストレーションに徹する) から切り出しています。
func (uc *DeleteAccountUsecase) deleteAccount(ctx context.Context, userID model.UserID, anonEmail, anonAtname string) error {
	tx, err := uc.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("トランザクションの開始に失敗: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	userRepo := uc.userRepo.WithTx(tx)
	userSessionRepo := uc.userSessionRepo.WithTx(tx)

	if err := userRepo.SoftDeleteAndAnonymize(ctx, userID, anonEmail, anonAtname); err != nil {
		return fmt.Errorf("ユーザーの論理削除・匿名化に失敗: %w", err)
	}
	if err := userSessionRepo.DeleteByUserID(ctx, userID); err != nil {
		return fmt.Errorf("ユーザーセッションの削除に失敗: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("トランザクションのコミットに失敗: %w", err)
	}
	return nil
}

// anonymizedEmail derives the placeholder email a withdrawn account's email is
// overwritten with. It embeds the user id so the value is globally unique (freeing
// the original address for re-registration without ever colliding on the
// users.email UNIQUE constraint), and uses the reserved .invalid TLD (RFC 2606) so
// it can never be a real, deliverable address.
//
// [Ja] anonymizedEmail は退会済みアカウントの email を上書きする代替 email を導出します。
// ユーザー id を埋め込むことで値をグローバルに一意にし (元のアドレスを再登録用に解放しつつ
// users.email の UNIQUE 制約で決して衝突しない)、予約 TLD の .invalid (RFC 2606) を使うことで
// 実在の配送可能なアドレスにならないようにします。
func anonymizedEmail(userID model.UserID) string {
	return fmt.Sprintf("deleted-%s@deleted.invalid", userID.String())
}

// anonymizedAtname derives the placeholder atname a withdrawn account's atname is
// overwritten with. It embeds the user id (with the UUID hyphens stripped so the
// value stays within the atname character set of ASCII letters/digits/underscore)
// so the value is globally unique, freeing the original atname for reuse without
// ever colliding on the users.atname UNIQUE constraint. The result is longer than
// the 20-character limit the account forms enforce, but that limit is form-level
// validation, not a column constraint (users.atname is citext with no length
// bound), and this tombstone value is never re-validated or shown as a handle.
//
// [Ja] anonymizedAtname は退会済みアカウントの atname を上書きする代替 atname を導出
// します。ユーザー id を埋め込む (UUID のハイフンを除いて atname の文字集合である ASCII
// 英数字 / アンダースコアに収める) ことで値をグローバルに一意にし、users.atname の UNIQUE
// 制約で決して衝突せずに元の atname を再利用向けに解放します。結果はアカウントフォームが
// 強制する 20 文字制限より長くなりますが、その制限はフォームレベルのバリデーションであって
// カラム制約ではなく (users.atname は長さ上限の無い citext)、この墓標値は再検証もハンドル
// としての表示もされません。
func anonymizedAtname(userID model.UserID) string {
	return "deleted_" + strings.ReplaceAll(userID.String(), "-", "")
}
