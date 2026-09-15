package usecase

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/groobb/groobb/go/internal/auth"
	"github.com/groobb/groobb/go/internal/dispatcher"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/validator"
)

// CreateEmailChangeUsecaseはメールアドレス変更申請を統括します。新しいアドレスと
// 現在のパスワードを検証し、新しいアドレス宛のメール確認コードを発行し、ユーザーに
// 紐付いた確認を永続化し、コードを届けるメールを投入します。users.emailは変更しません。
// アドレスの切り替えはコード検証後のステップで行われるため、本ステップは新しい
// アドレスが到達可能で、申請が現在のパスワードで認可されていることを確認するだけです。
type CreateEmailChangeUsecase struct {
	writer                 *sql.DB
	settingsEmailValidator *validator.SettingsEmailUpdateValidator
	emailConfirmationRepo  *repository.EmailConfirmationRepository
	dispatcher             *dispatcher.Dispatcher
}

// NewCreateEmailChangeUsecaseは書き込み用プール・validator・永続化に使うリポジトリ・
// dispatcherからCreateEmailChangeUsecaseを構築します。
func NewCreateEmailChangeUsecase(
	writer *sql.DB,
	settingsEmailValidator *validator.SettingsEmailUpdateValidator,
	emailConfirmationRepo *repository.EmailConfirmationRepository,
	dispatcher *dispatcher.Dispatcher,
) *CreateEmailChangeUsecase {
	return &CreateEmailChangeUsecase{
		writer:                 writer,
		settingsEmailValidator: settingsEmailValidator,
		emailConfirmationRepo:  emailConfirmationRepo,
		dispatcher:             dispatcher,
	}
}

// CreateEmailChangeInputはExecuteの入力です。UserIDは変更を申請するサインイン
// 済みユーザー、NewEmailとCurrentPasswordは送信されたフォーム値、Localeはリクエストの
// ロケールで、確認メールをユーザーが閲覧中の言語で描画するために運びます。
type CreateEmailChangeInput struct {
	UserID          model.UserID
	NewEmail        string
	CurrentPassword string
	Locale          model.Locale
}

// CreateEmailChangeOutputは作成された確認を運びます。確認ステップは (受け渡し
// Cookieではなく) セッションのユーザーから確認を引くため、ハンドラーはフローを進める
// のに本値を必要としません。テストや後続の呼び出し元が発行内容を観測できるよう返します。
type CreateEmailChangeOutput struct {
	EmailConfirmation *model.EmailConfirmation
}

// Executeは入力を検証してから確認を発行します。バリデーションを先に走らせ、
// 不正・未変更・重複のemailや誤った現在のパスワードでは行を作らずに
// *model.ValidationErrorを返します。
func (uc *CreateEmailChangeUsecase) Execute(ctx context.Context, input CreateEmailChangeInput) (*CreateEmailChangeOutput, error) {
	if err := uc.settingsEmailValidator.Validate(ctx, validator.SettingsEmailUpdateValidatorInput{
		UserID:          input.UserID,
		NewEmail:        input.NewEmail,
		CurrentPassword: input.CurrentPassword,
	}); err != nil {
		return nil, err
	}

	return uc.createEmailChange(ctx, input)
}

// createEmailChangeはコードを生成し、保留中のメール変更の確認を1トランザクション
// で新しいものに置き換えてから、メールを投入します。コード生成はトランザクションの前に
// 行い、トランザクションが永続化のみを保持するようにします。削除してから作成する処理は
// トランザクション化し、作成に失敗した場合は削除もロールバックして、既存の保留中確認を
// 維持します。メールはコミット後に投入します。ジョブキューは別プールで動き、
// 本トランザクションの外にあるためです。
func (uc *CreateEmailChangeUsecase) createEmailChange(ctx context.Context, input CreateEmailChangeInput) (*CreateEmailChangeOutput, error) {
	code, err := auth.GenerateConfirmationCode()
	if err != nil {
		return nil, fmt.Errorf("確認コードの生成に失敗: %w", err)
	}

	tx, err := uc.writer.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("トランザクションの開始に失敗: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	emailConfirmationRepo := uc.emailConfirmationRepo.WithTx(tx)

	if err := emailConfirmationRepo.DeleteUnusedEmailChangesByUserID(ctx, input.UserID); err != nil {
		return nil, fmt.Errorf("保留中のメール変更確認の削除に失敗: %w", err)
	}

	confirmation, err := emailConfirmationRepo.CreateEmailChange(ctx, repository.CreateEmailChangeInput{
		UserID: input.UserID,
		Email:  input.NewEmail,
		Code:   code,
	})
	if err != nil {
		return nil, fmt.Errorf("メール変更確認の作成に失敗: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("トランザクションのコミットに失敗: %w", err)
	}

	// 確認メールを投入する。ここでの失敗は握り潰さずAppErrorとして表面化する。
	// コードを届けられないのにユーザーをコード入力ステップへ進めると、届かないメールを
	// 待ち続けて手詰まりになるため。エラーを返すことでハンドラーはユーザーを変更フォームに
	// 留めて再申請させられる。内部原因と対象emailはログ用にのみ添える。
	if err := uc.dispatcher.EnqueueEmailConfirmation(ctx, input.NewEmail, code, input.Locale); err != nil {
		return nil, &model.AppError{
			Code:     model.AppErrCodeInternal,
			UserMsg:  i18n.T(ctx, "validation_email_delivery_failed"),
			Internal: fmt.Errorf("確認メールのジョブ投入に失敗: %w", err),
			Metadata: map[string]string{"email": input.NewEmail},
		}
	}

	return &CreateEmailChangeOutput{EmailConfirmation: confirmation}, nil
}
