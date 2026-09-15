package usecase

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/groobb/groobb/go/internal/dispatcher"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/validator"
)

// VerifyEmailChangeUsecaseはメール変更の確認ステップを統括します。送信された
// コードの形式を検証し、サインイン済みユーザーのアクティブなメール変更の確認を解決し、
// コードを照合し、正しいコードなら新しいアドレスをusers.emailに適用します。これにより
// 変更は、ユーザーが新しいアドレスの管理権を証明したときにのみ成立します。誤ったコードは
// 確認の失敗試行回数をインクリメントして繰り返しの推測に上限を設けます。照合・打刻・
// メール更新は1トランザクションで行います。失敗経路 (インクリメント) も成功経路 (打刻 +
// 更新) も書き込み、かつ打刻と更新は一緒に成立するか全く成立しないかでなければならない
// ためです。本UseCaseは検証と適用の両方を行います (検証とアカウント作成が別ステップの
// サインアップと異なります)。確認自体が切り替え先の新しいアドレスを持つためです。変更の
// コミット後、以前のアドレスへベストエフォートの通知を投入し、本人が意図しない変更に
// 気づけるようにします。
type VerifyEmailChangeUsecase struct {
	writer                *sql.DB
	confirmationValidator *validator.SettingsEmailConfirmationCreateValidator
	emailConfirmationRepo *repository.EmailConfirmationRepository
	userRepo              *repository.UserRepository
	dispatcher            *dispatcher.Dispatcher
}

// NewVerifyEmailChangeUsecaseは書き込み用プール・validator・永続化に使うリポジトリ・変更後の
// 通知を投入するdispatcherからVerifyEmailChangeUsecaseを構築します。
func NewVerifyEmailChangeUsecase(
	writer *sql.DB,
	confirmationValidator *validator.SettingsEmailConfirmationCreateValidator,
	emailConfirmationRepo *repository.EmailConfirmationRepository,
	userRepo *repository.UserRepository,
	dispatcher *dispatcher.Dispatcher,
) *VerifyEmailChangeUsecase {
	return &VerifyEmailChangeUsecase{
		writer:                writer,
		confirmationValidator: confirmationValidator,
		emailConfirmationRepo: emailConfirmationRepo,
		userRepo:              userRepo,
		dispatcher:            dispatcher,
	}
}

// VerifyEmailChangeInputはExecuteの入力です。UserIDはコードを送信するサインイン
// 済みユーザー、Codeはユーザーが入力した値です。
type VerifyEmailChangeInput struct {
	UserID model.UserID
	Code   string
}

// VerifyEmailChangeOutputは検証済みの確認を運びます。そのEmailはたった今ユーザーに
// 適用されたアドレスです。ハンドラーはフローを進めるのにこれを必要としません
// (リダイレクトするため) が、テストや後続の呼び出し元が何が変わったかを観測できるよう
// 返します。
type VerifyEmailChangeOutput struct {
	EmailConfirmation *model.EmailConfirmation
}

// Executeはコードの形式を検証し、アクティブな確認を解決し、変更を適用してから、
// 旧アドレスへの通知を投入します。バリデーション (形式 + アクティブな確認の取得) と
// ユーザーの取得はデータ取得としてトランザクション外で行い、照合・打刻・更新のみが
// トランザクションのステップです。ユーザーは変更前に取得し、現在の (旧) アドレス
// (更新が上書きするため通知先となる) とアカウントのロケールを捉えます。通知は変更が
// コミットされた後にのみ投入し、ベストエフォートとします。アドレスは既に切り替わって
// いるため、投入失敗は表面化させずログに記録します (エラーを返すと変更が起きなかったかの
// ように誤って示唆してしまうため)。
func (uc *VerifyEmailChangeUsecase) Execute(ctx context.Context, input VerifyEmailChangeInput) (*VerifyEmailChangeOutput, error) {
	confirmation, err := uc.confirmationValidator.Validate(ctx, validator.SettingsEmailConfirmationCreateValidatorInput{
		UserID: input.UserID,
		Code:   input.Code,
	})
	if err != nil {
		return nil, err
	}

	// メール変更では確認のUserIDは非nilであり、FKのcascadeが行の存在を保証する。
	// ここでnilなら不変条件が壊れているため、状態を変える前に内部エラーとして扱う。
	user, err := uc.userRepo.FindByID(ctx, *confirmation.UserID)
	if err != nil {
		return nil, fmt.Errorf("メール変更確認に紐づくユーザーの取得に失敗: %w", err)
	}
	if user == nil {
		return nil, fmt.Errorf("メール変更確認に紐づくユーザーが存在しません: user_id=%s", confirmation.UserID)
	}
	oldEmail := user.Email
	locale := user.Locale

	output, err := uc.verify(ctx, confirmation, input.Code)
	if err != nil {
		return nil, err
	}

	uc.notifyOldAddress(ctx, oldEmail, confirmation.Email, locale)

	return output, nil
}

// notifyOldAddressはユーザーの以前のアドレスへ変更通知を投入する。変更が
// コミットされた後にのみ実行し、ベストエフォートとする。投入失敗は (調査用に宛先を
// 添えて) ログに記録するが返さないため、完了済みの変更は成功として報告される。メールは
// アカウントに保存されたロケールで描画する。通知は現在のリクエストの言語に紐づくのでは
// なく、アカウントの所有者に宛てるものだからである。
func (uc *VerifyEmailChangeUsecase) notifyOldAddress(ctx context.Context, oldEmail, newEmail string, locale model.Locale) {
	if err := uc.dispatcher.EnqueueEmailChangeNotification(ctx, oldEmail, newEmail, locale); err != nil {
		slog.ErrorContext(ctx, "メールアドレス変更通知メールのジョブ投入に失敗", "error", err, "email", oldEmail)
	}
}

// verifyは送信されたコードをアクティブな確認と照合し、1トランザクションで、失敗
// 試行回数をインクリメントする (誤ったコード) か、確認を成功済みとして打刻し新しいアドレスを
// users.emailに適用する (正しいコード) かのいずれかを行います。誤ったコードはインクリメントを
// コミットし、validatorの未検出 / 期限切れメッセージと一致するフォーム全体のValidationErrorを
// 返すため、フォームは誤ったコードと確認の不在を区別しません (列挙攻撃対策)。正しいコードでは
// 打刻とメール更新がトランザクションを共有するため、どちらかの失敗は両方をロールバックします。
// 更新時のUNIQUE違反は、申請時のチェックから現在までの間に新しいアドレスが取得されたことを
// 意味し、フォーム全体のValidationError (500ではなく) として表面化させ、ユーザーが別の
// アドレスで再試行できるようにします。
func (uc *VerifyEmailChangeUsecase) verify(ctx context.Context, confirmation *model.EmailConfirmation, code string) (*VerifyEmailChangeOutput, error) {
	tx, err := uc.writer.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("トランザクションの開始に失敗: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	emailConfirmationRepo := uc.emailConfirmationRepo.WithTx(tx)
	userRepo := uc.userRepo.WithTx(tx)

	if confirmation.Code != code {
		if err := emailConfirmationRepo.IncrementFailedAttempts(ctx, confirmation.ID); err != nil {
			return nil, fmt.Errorf("メール変更確認の失敗試行回数の更新に失敗: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("トランザクションのコミットに失敗: %w", err)
		}

		ve := model.NewValidationError()
		ve.AddGlobal(i18n.T(ctx, "validation_code_incorrect_or_expired"))
		return nil, ve
	}

	if err := emailConfirmationRepo.Succeed(ctx, confirmation.ID); err != nil {
		return nil, fmt.Errorf("メール変更確認の成功打刻に失敗: %w", err)
	}

	// UserIDはメール変更の確認では非nilであり (サインイン済みユーザーが発行する)、
	// Emailは切り替え先の新しいアドレスである。
	if err := userRepo.UpdateEmail(ctx, *confirmation.UserID, confirmation.Email); err != nil {
		// メール変更の適用は、申請時の一意性チェックからこの更新までの間に新しい
		// アドレスが別アカウントに取得されるとUNIQUE制約に当たる。その競合を500では
		// なくユーザーが修正できるバリデーションエラーに変換する。
		if repository.IsUniqueViolation(err) {
			// 申請時のチェックの後に別アカウントがアドレスを取得した。コミット
			// せずに返し、deferが打刻と更新の両方をロールバックして確認をactiveの
			// まま残す。ユーザーは別のアドレスで再試行するよう促される。
			ve := model.NewValidationError()
			ve.AddGlobal(i18n.T(ctx, "validation_email_change_conflict"))
			return nil, ve
		}
		return nil, fmt.Errorf("メールアドレスの更新に失敗: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("トランザクションのコミットに失敗: %w", err)
	}

	return &VerifyEmailChangeOutput{EmailConfirmation: confirmation}, nil
}
