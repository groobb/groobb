package usecase

import (
	"context"
	"database/sql"
	"fmt"

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
	writer          *sql.DB
	validator       *validator.SettingsWithdrawalDeleteValidator
	userRepo        *repository.UserRepository
	userSessionRepo *repository.UserSessionRepository
}

// NewDeleteAccountUsecase builds a DeleteAccountUsecase from the write pool, its
// validator, and the repositories it persists through.
//
// [Ja] NewDeleteAccountUsecase は書き込み用プール・validator・永続化に使うリポジトリから
// DeleteAccountUsecase を構築します。
func NewDeleteAccountUsecase(
	writer *sql.DB,
	validator *validator.SettingsWithdrawalDeleteValidator,
	userRepo *repository.UserRepository,
	userSessionRepo *repository.UserSessionRepository,
) *DeleteAccountUsecase {
	return &DeleteAccountUsecase{
		writer:          writer,
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

	return uc.deleteAccount(ctx, input.UserID, model.AnonymizedEmail(input.UserID), model.AnonymizedAtname(input.UserID))
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
	tx, err := uc.writer.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("トランザクションの開始に失敗: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	userRepo := uc.userRepo.WithTx(tx)
	userSessionRepo := uc.userSessionRepo.WithTx(tx)

	if err := userRepo.SoftDeleteAndAnonymize(ctx, userID, anonEmail, anonAtname); err != nil {
		return fmt.Errorf("ユーザーの論理削除・匿名化に失敗: %w", err)
	}
	if err := userSessionRepo.DeleteByUserID(ctx, userID); err != nil {
		return fmt.Errorf("ユーザーセッションの削除に失敗: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("トランザクションのコミットに失敗: %w", err)
	}
	return nil
}
