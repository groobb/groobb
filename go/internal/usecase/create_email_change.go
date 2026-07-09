package usecase

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/groobb/groobb/go/internal/auth"
	"github.com/groobb/groobb/go/internal/dispatcher"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/validator"
)

// CreateEmailChangeUsecase orchestrates an email-change request: it validates the
// new address and the current password, issues an email confirmation code for the
// new address, persists the confirmation tied to the user, and enqueues the mail
// that delivers the code. It does not change users.email; the address is switched
// only after the code is verified (a later task), so this step just proves the
// new address is reachable and the request is authorized by the current password.
//
// [Ja] CreateEmailChangeUsecase はメールアドレス変更申請を統括します。新しいアドレスと
// 現在のパスワードを検証し、新しいアドレス宛のメール確認コードを発行し、ユーザーに
// 紐付いた確認を永続化し、コードを届けるメールを投入します。users.email は変更しません。
// アドレスの切り替えはコード検証後 (後続タスク) に初めて行われるため、本ステップは新しい
// アドレスが到達可能で、申請が現在のパスワードで認可されていることを確認するだけです。
type CreateEmailChangeUsecase struct {
	db                     *pgxpool.Pool
	settingsEmailValidator *validator.SettingsEmailUpdateValidator
	emailConfirmationRepo  *repository.EmailConfirmationRepository
	dispatcher             *dispatcher.Dispatcher
}

// NewCreateEmailChangeUsecase builds a CreateEmailChangeUsecase from the pool, its
// validator, the repository it persists through, and the dispatcher.
//
// [Ja] NewCreateEmailChangeUsecase はプール・validator・永続化に使うリポジトリ・
// dispatcher から CreateEmailChangeUsecase を構築します。
func NewCreateEmailChangeUsecase(
	db *pgxpool.Pool,
	settingsEmailValidator *validator.SettingsEmailUpdateValidator,
	emailConfirmationRepo *repository.EmailConfirmationRepository,
	dispatcher *dispatcher.Dispatcher,
) *CreateEmailChangeUsecase {
	return &CreateEmailChangeUsecase{
		db:                     db,
		settingsEmailValidator: settingsEmailValidator,
		emailConfirmationRepo:  emailConfirmationRepo,
		dispatcher:             dispatcher,
	}
}

// CreateEmailChangeInput is the input to Execute. UserID is the signed-in user
// requesting the change; NewEmail and CurrentPassword are the submitted form
// values; Locale is the request locale, carried so the confirmation mail is
// rendered in the language the user is browsing in.
//
// [Ja] CreateEmailChangeInput は Execute の入力です。UserID は変更を申請するサインイン
// 済みユーザー、NewEmail と CurrentPassword は送信されたフォーム値、Locale はリクエストの
// ロケールで、確認メールをユーザーが閲覧中の言語で描画するために運びます。
type CreateEmailChangeInput struct {
	UserID          model.UserID
	NewEmail        string
	CurrentPassword string
	Locale          string
}

// CreateEmailChangeOutput carries the created confirmation. The confirm step
// looks the confirmation up by the session user (not a handoff cookie), so the
// handler does not need this to advance the flow; it is returned so tests and any
// later caller can observe what was issued.
//
// [Ja] CreateEmailChangeOutput は作成された確認を運びます。確認ステップは (受け渡し
// Cookie ではなく) セッションのユーザーから確認を引くため、ハンドラーはフローを進める
// のに本値を必要としません。テストや後続の呼び出し元が発行内容を観測できるよう返します。
type CreateEmailChangeOutput struct {
	EmailConfirmation *model.EmailConfirmation
}

// Execute validates the input and then issues the confirmation. Validation runs
// first so an invalid, unchanged, or duplicate email, or a wrong current
// password, returns a *model.ValidationError without creating any row.
//
// [Ja] Execute は入力を検証してから確認を発行します。バリデーションを先に走らせ、
// 不正・未変更・重複の email や誤った現在のパスワードでは行を作らずに
// *model.ValidationError を返します。
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

// createEmailChange generates the code, replaces any pending email-change
// confirmation with a fresh one in a single transaction, then enqueues the mail.
// The code generation runs before the transaction so the transaction holds only
// persistence. The delete-then-create is transactional so the "at most one
// pending email change per user" invariant holds even if the create fails
// (leaving the user with none rather than the old one). The mail is enqueued
// after the commit because the job queue runs on a separate pool, outside this
// transaction.
//
// [Ja] createEmailChange はコードを生成し、保留中のメール変更の確認を 1 トランザクション
// で新しいものに置き換えてから、メールを投入します。コード生成はトランザクションの前に
// 行い、トランザクションが永続化のみを保持するようにします。削除してから作成する処理は
// トランザクション化し、作成が失敗しても「ユーザーごとに保留中のメール変更は高々 1 件」の
// 不変条件が保たれる (古いものではなく 0 件が残る) ようにします。メールはコミット後に投入
// します。ジョブキューは別プールで動き、本トランザクションの外にあるためです。
func (uc *CreateEmailChangeUsecase) createEmailChange(ctx context.Context, input CreateEmailChangeInput) (*CreateEmailChangeOutput, error) {
	code, err := auth.GenerateConfirmationCode()
	if err != nil {
		return nil, fmt.Errorf("確認コードの生成に失敗: %w", err)
	}

	tx, err := uc.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("トランザクションの開始に失敗: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

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

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("トランザクションのコミットに失敗: %w", err)
	}

	// Enqueue the confirmation mail. A failure here is surfaced as an AppError, not
	// swallowed: if the code can never be delivered, advancing the user to the
	// code-entry step would strand them waiting for a mail that will not arrive.
	// Returning the error lets the handler keep them on the change form to retry.
	// The internal cause and the affected email are attached for logging only.
	//
	// [Ja] 確認メールを投入する。ここでの失敗は握り潰さず AppError として表面化する。
	// コードを届けられないのにユーザーをコード入力ステップへ進めると、届かないメールを
	// 待ち続けて手詰まりになるため。エラーを返すことでハンドラーはユーザーを変更フォームに
	// 留めて再申請させられる。内部原因と対象 email はログ用にのみ添える。
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
