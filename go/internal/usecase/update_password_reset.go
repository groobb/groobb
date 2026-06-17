package usecase

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/groobb/groobb/go/internal/auth"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/validator"
)

// UpdatePasswordResetUsecase orchestrates setting a new password from a reset
// link: it validates the token and the chosen password, then in one transaction
// replaces the user's password and marks the token used. Both writes share a
// transaction so a link can never be spent without the password actually changing
// (or vice versa). The user is identified by the token, not a session, because a
// password reset happens while signed out.
//
// [Ja] UpdatePasswordResetUsecase はリセットリンクからの新パスワード設定を統括します。
// トークンと選んだパスワードを検証し、1 トランザクションでユーザーのパスワードを置き換えて
// トークンを使用済みにします。両方の書き込みが同一トランザクションを共有するため、パスワードが
// 実際に変わらないままリンクが消費される (またはその逆) ことが決して起きません。パスワード
// リセットはサインアウト中に行われるため、ユーザーはセッションではなくトークンで特定します。
type UpdatePasswordResetUsecase struct {
	db                     *pgxpool.Pool
	updateValidator        *validator.PasswordUpdateValidator
	passwordResetTokenRepo *repository.PasswordResetTokenRepository
	userPasswordRepo       *repository.UserPasswordRepository
}

// NewUpdatePasswordResetUsecase builds an UpdatePasswordResetUsecase from the
// pool, the validator, and the repositories it persists through.
//
// [Ja] NewUpdatePasswordResetUsecase はプール・validator・永続化に使うリポジトリから
// UpdatePasswordResetUsecase を構築します。
func NewUpdatePasswordResetUsecase(
	db *pgxpool.Pool,
	updateValidator *validator.PasswordUpdateValidator,
	passwordResetTokenRepo *repository.PasswordResetTokenRepository,
	userPasswordRepo *repository.UserPasswordRepository,
) *UpdatePasswordResetUsecase {
	return &UpdatePasswordResetUsecase{
		db:                     db,
		updateValidator:        updateValidator,
		passwordResetTokenRepo: passwordResetTokenRepo,
		userPasswordRepo:       userPasswordRepo,
	}
}

// UpdatePasswordResetInput is the input to Execute. Token is the plaintext reset
// token from the link; Password and PasswordConfirmation are the new credential.
//
// [Ja] UpdatePasswordResetInput は Execute の入力です。Token はリンクから来る平文の
// リセットトークン、Password / PasswordConfirmation は新しい資格情報です。
type UpdatePasswordResetInput struct {
	Token                string
	Password             string
	PasswordConfirmation string
}

// Execute validates the token and password, hashes the new password, and then in
// one transaction updates the credential and spends the token. Password hashing
// runs before the transaction so bcrypt's cost is not paid while holding a row
// lock.
//
// [Ja] Execute はトークンとパスワードを検証し、新しいパスワードをハッシュ化し、その後
// 1 トランザクションで資格情報を更新しトークンを消費します。パスワードのハッシュ化は、
// 行ロックを保持したまま bcrypt のコストを払わないよう、トランザクションの前に実行します。
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

// updatePassword replaces the user's password and marks the reset token used in a
// single transaction, so the link is spent exactly when the new password takes
// effect. The digest is computed by Execute beforehand, keeping the transaction
// to pure persistence.
//
// [Ja] updatePassword はユーザーのパスワードを置き換えリセットトークンを使用済みにする
// 処理を 1 トランザクションで行い、新しいパスワードが有効になるのとちょうど同時にリンクが
// 消費されるようにします。ダイジェストは事前に Execute が計算済みで、トランザクションを
// 純粋な永続化に保ちます。
func (uc *UpdatePasswordResetUsecase) updatePassword(ctx context.Context, validated *validator.PasswordUpdateValidateOutput, passwordDigest string) error {
	tx, err := uc.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("トランザクションの開始に失敗: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	userPasswordRepo := uc.userPasswordRepo.WithTx(tx)
	passwordResetTokenRepo := uc.passwordResetTokenRepo.WithTx(tx)

	if err := userPasswordRepo.UpdatePasswordDigest(ctx, validated.UserID, passwordDigest); err != nil {
		return fmt.Errorf("パスワードの更新に失敗: %w", err)
	}

	if err := passwordResetTokenRepo.MarkAsUsed(ctx, validated.TokenID); err != nil {
		return fmt.Errorf("リセットトークンの使用済みマークに失敗: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("トランザクションのコミットに失敗: %w", err)
	}

	return nil
}
