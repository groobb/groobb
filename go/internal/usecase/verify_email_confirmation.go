package usecase

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/validator"
)

// VerifyEmailConfirmationUsecaseは確認コードの検証を統括します。送信された
// コードの形式を検証し、アクティブな確認を解決し、コードを照合します。正しいコードは
// 確認を成功済みとして打刻し、誤ったコードは失敗試行回数をインクリメントして繰り返しの
// 推測に上限を設けます。失敗経路がDBに書き込むため、照合と書き込みはトランザクション内で
// 行います (試行回数を伴うコード検証についてのバリデーションガイドラインに従う)。
// ユーザーは作成しません。アカウントは成功済みの確認を読む後続ステップ (アカウント作成)
// で作られるため、本ステップはコードが正しかったことを確認するだけです。
type VerifyEmailConfirmationUsecase struct {
	writer                     *sql.DB
	emailConfirmationValidator *validator.EmailConfirmationCreateValidator
	emailConfirmationRepo      *repository.EmailConfirmationRepository
}

// NewVerifyEmailConfirmationUsecaseは書き込み用プール・validator・repositoryから
// VerifyEmailConfirmationUsecaseを構築します。
func NewVerifyEmailConfirmationUsecase(
	writer *sql.DB,
	emailConfirmationValidator *validator.EmailConfirmationCreateValidator,
	emailConfirmationRepo *repository.EmailConfirmationRepository,
) *VerifyEmailConfirmationUsecase {
	return &VerifyEmailConfirmationUsecase{
		writer:                     writer,
		emailConfirmationValidator: emailConfirmationValidator,
		emailConfirmationRepo:      emailConfirmationRepo,
	}
}

// VerifyEmailConfirmationInputはExecuteの入力です。IDは保留中の確認のid
// (受け渡しCookieから運ばれる)、Codeはユーザーが入力した値です。
type VerifyEmailConfirmationInput struct {
	ID   model.EmailConfirmationID
	Code string
}

// VerifyEmailConfirmationOutputは検証済みの確認を運び、ハンドラーが次にユーザーを
// どこへ送るかを決められるようにします (emailはアカウント作成で再利用します)。
type VerifyEmailConfirmationOutput struct {
	EmailConfirmation *model.EmailConfirmation
}

// Executeはコードの形式を検証し、アクティブな確認を解決した後、verifyに委譲して
// コードを照合し結果を永続化します。バリデーション (形式 + アクティブな確認の取得) は
// データ取得としてトランザクション外で行い、照合と書き込みのみがトランザクションの
// ステップです。
func (uc *VerifyEmailConfirmationUsecase) Execute(ctx context.Context, input VerifyEmailConfirmationInput) (*VerifyEmailConfirmationOutput, error) {
	confirmation, err := uc.emailConfirmationValidator.Validate(ctx, validator.EmailConfirmationCreateValidatorInput{
		ID:   input.ID,
		Code: input.Code,
	})
	if err != nil {
		return nil, err
	}

	return uc.verify(ctx, confirmation, input.Code)
}

// verifyは送信されたコードをアクティブな確認と照合し、1トランザクションで、確認を
// 成功済みとして打刻する (正しいコード) か、失敗試行回数をインクリメントする (誤った
// コード) かのいずれかを行います。誤ったコードはインクリメントをコミットしてから
// フォーム全体のValidationErrorを返します。このメッセージはvalidatorの未検出 /
// 期限切れメッセージと一致するため、フォームは誤ったコードと確認の不在を区別しません
// (列挙攻撃対策)。インクリメントは単一のアトミックなUPDATEのため、並行する誤った推測も
// 数えられ、カウントは上限を超えて収束し、それ以降はFindActiveByIDが当該行を返さなく
// なります。
func (uc *VerifyEmailConfirmationUsecase) verify(ctx context.Context, confirmation *model.EmailConfirmation, code string) (*VerifyEmailConfirmationOutput, error) {
	tx, err := uc.writer.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("トランザクションの開始に失敗: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	emailConfirmationRepo := uc.emailConfirmationRepo.WithTx(tx)

	if confirmation.Code != code {
		if err := emailConfirmationRepo.IncrementFailedAttempts(ctx, confirmation.ID); err != nil {
			return nil, fmt.Errorf("メール確認の失敗試行回数の更新に失敗: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("トランザクションのコミットに失敗: %w", err)
		}

		ve := model.NewValidationError()
		ve.AddGlobal(i18n.T(ctx, "validation_code_incorrect_or_expired"))
		return nil, ve
	}

	if err := emailConfirmationRepo.Succeed(ctx, confirmation.ID); err != nil {
		return nil, fmt.Errorf("メール確認の成功打刻に失敗: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("トランザクションのコミットに失敗: %w", err)
	}

	return &VerifyEmailConfirmationOutput{EmailConfirmation: confirmation}, nil
}
