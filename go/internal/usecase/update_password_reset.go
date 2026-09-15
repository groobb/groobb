package usecase

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/groobb/groobb/go/internal/auth"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/validator"
)

// UpdatePasswordResetUsecaseはリセットリンクからの新パスワード設定を統括します。
// トークンと選んだパスワードを検証し、1トランザクションでユーザーのパスワードを置き換えて
// トークンを使用済みにします。両方の書き込みが同一トランザクションを共有するため、パスワードが
// 実際に変わらないままリンクが消費される (またはその逆) ことが決して起きません。パスワード
// リセットはサインアウト中に行われるため、ユーザーはセッションではなくトークンで特定します。
type UpdatePasswordResetUsecase struct {
	writer                 *sql.DB
	updateValidator        *validator.PasswordUpdateValidator
	passwordResetTokenRepo *repository.PasswordResetTokenRepository
	userPasswordRepo       *repository.UserPasswordRepository
}

// NewUpdatePasswordResetUsecaseは書き込み用プール・validator・永続化に使うリポジトリから
// UpdatePasswordResetUsecaseを構築します。
func NewUpdatePasswordResetUsecase(
	writer *sql.DB,
	updateValidator *validator.PasswordUpdateValidator,
	passwordResetTokenRepo *repository.PasswordResetTokenRepository,
	userPasswordRepo *repository.UserPasswordRepository,
) *UpdatePasswordResetUsecase {
	return &UpdatePasswordResetUsecase{
		writer:                 writer,
		updateValidator:        updateValidator,
		passwordResetTokenRepo: passwordResetTokenRepo,
		userPasswordRepo:       userPasswordRepo,
	}
}

// UpdatePasswordResetInputはExecuteの入力です。Tokenはリンクから来る平文の
// リセットトークン、Password / PasswordConfirmationは新しい資格情報です。
type UpdatePasswordResetInput struct {
	Token                string
	Password             string
	PasswordConfirmation string
}

// Executeはトークンとパスワードを検証し、新しいパスワードをハッシュ化し、その後
// 1トランザクションで資格情報を更新しトークンを消費します。パスワードのハッシュ化は、
// SQLiteの書き込みロックを保持したままbcryptのコストを払わないよう、トランザクションの前に実行します。
func (uc *UpdatePasswordResetUsecase) Execute(ctx context.Context, input UpdatePasswordResetInput) error {
	validated, err := uc.updateValidator.Validate(ctx, validator.PasswordUpdateValidatorInput{
		Token:                input.Token,
		Password:             input.Password,
		PasswordConfirmation: input.PasswordConfirmation,
	})
	if err != nil {
		return err
	}

	passwordDigest, err := auth.HashPassword(input.Password)
	if err != nil {
		return fmt.Errorf("パスワードのハッシュ化に失敗: %w", err)
	}

	return uc.updatePassword(ctx, validated, passwordDigest)
}

// updatePasswordはユーザーのパスワードを置き換えリセットトークンを使用済みにする
// 処理を1トランザクションで行い、新しいパスワードが有効になるのとちょうど同時にリンクが
// 消費されるようにします。ダイジェストは事前にExecuteが計算済みで、トランザクションを
// 純粋な永続化に保ちます。
func (uc *UpdatePasswordResetUsecase) updatePassword(ctx context.Context, validated *validator.PasswordUpdateValidateOutput, passwordDigest string) error {
	tx, err := uc.writer.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("トランザクションの開始に失敗: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	userPasswordRepo := uc.userPasswordRepo.WithTx(tx)
	passwordResetTokenRepo := uc.passwordResetTokenRepo.WithTx(tx)

	if err := userPasswordRepo.UpdatePasswordDigest(ctx, validated.UserID, passwordDigest); err != nil {
		return fmt.Errorf("パスワードの更新に失敗: %w", err)
	}

	if err := passwordResetTokenRepo.MarkAsUsed(ctx, validated.TokenID); err != nil {
		return fmt.Errorf("リセットトークンの使用済みマークに失敗: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("トランザクションのコミットに失敗: %w", err)
	}

	return nil
}
