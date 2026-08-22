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

// VerifyEmailChangeUsecase orchestrates the confirm step of an email change: it
// validates the submitted code's format, resolves the signed-in user's active
// email-change confirmation, compares the code, and on a correct code applies the
// new address to users.email — all so the change lands only after the user proves
// control of the new address. A wrong code increments the confirmation's
// failed-attempt count so repeated guessing is capped. The compare, stamp, and
// email update run in one transaction because both the failure path (the
// increment) and the success path (stamp + update) write, and the stamp and
// update must land together or not at all. This usecase both verifies and applies
// (unlike sign-up, where verification and account creation are separate steps),
// because the confirmation itself carries the new address to switch to. After the
// change commits, it enqueues a best-effort notification to the previous address
// so the account owner learns of a change they may not have made.
//
// [Ja] VerifyEmailChangeUsecase はメール変更の確認ステップを統括します。送信された
// コードの形式を検証し、サインイン済みユーザーのアクティブなメール変更の確認を解決し、
// コードを照合し、正しいコードなら新しいアドレスを users.email に適用します。これにより
// 変更は、ユーザーが新しいアドレスの管理権を証明したときにのみ成立します。誤ったコードは
// 確認の失敗試行回数をインクリメントして繰り返しの推測に上限を設けます。照合・打刻・
// メール更新は 1 トランザクションで行います。失敗経路 (インクリメント) も成功経路 (打刻 +
// 更新) も書き込み、かつ打刻と更新は一緒に成立するか全く成立しないかでなければならない
// ためです。本 UseCase は検証と適用の両方を行います (検証とアカウント作成が別ステップの
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

// NewVerifyEmailChangeUsecase builds a VerifyEmailChangeUsecase from the write pool, its
// validator, the repositories it persists through, and the dispatcher used to
// enqueue the post-change notification.
//
// [Ja] NewVerifyEmailChangeUsecase は書き込み用プール・validator・永続化に使うリポジトリ・変更後の
// 通知を投入する dispatcher から VerifyEmailChangeUsecase を構築します。
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

// VerifyEmailChangeInput is the input to Execute. UserID is the signed-in user
// submitting the code, and Code is the value the user typed.
//
// [Ja] VerifyEmailChangeInput は Execute の入力です。UserID はコードを送信するサインイン
// 済みユーザー、Code はユーザーが入力した値です。
type VerifyEmailChangeInput struct {
	UserID model.UserID
	Code   string
}

// VerifyEmailChangeOutput carries the verified confirmation, whose Email is the
// address just applied to the user. The handler does not need it to advance the
// flow (it redirects), but it is returned so tests and any later caller can
// observe what was changed.
//
// [Ja] VerifyEmailChangeOutput は検証済みの確認を運びます。その Email はたった今ユーザーに
// 適用されたアドレスです。ハンドラーはフローを進めるのにこれを必要としません
// (リダイレクトするため) が、テストや後続の呼び出し元が何が変わったかを観測できるよう
// 返します。
type VerifyEmailChangeOutput struct {
	EmailConfirmation *model.EmailConfirmation
}

// Execute validates the code's format, resolves the active confirmation, applies
// the change, then enqueues the notification to the old address. Validation
// (format + active-confirmation lookup) and the user lookup run outside the
// transaction as data retrieval; the compare-stamp-and-update is the only
// transactional step. The user is fetched before the change so the current (old)
// address — the one to notify, since the update overwrites it — and the account
// locale are captured. The notification is enqueued only after the change commits
// and is best-effort: the address already switched, so a failed enqueue is logged
// rather than surfaced (returning an error would wrongly imply the change did not
// happen).
//
// [Ja] Execute はコードの形式を検証し、アクティブな確認を解決し、変更を適用してから、
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

	// The confirmation's UserID is non-nil for an email change and the FK cascade
	// guarantees the row exists; a nil here would be a broken invariant, so treat
	// it as an internal error before any state changes.
	//
	// [Ja] メール変更では確認の UserID は非 nil であり、FK の cascade が行の存在を保証する。
	// ここで nil なら不変条件が壊れているため、状態を変える前に内部エラーとして扱う。
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

// notifyOldAddress enqueues the change notification to the user's previous
// address. It runs only after the change has committed and is best-effort: a
// failed enqueue is logged (with the recipient for investigation) but not
// returned, so the completed change is still reported as success. The mail is
// rendered in the account's stored locale, since the notification is addressed to
// the account owner rather than tied to the current request's language.
//
// [Ja] notifyOldAddress はユーザーの以前のアドレスへ変更通知を投入する。変更が
// コミットされた後にのみ実行し、ベストエフォートとする。投入失敗は (調査用に宛先を
// 添えて) ログに記録するが返さないため、完了済みの変更は成功として報告される。メールは
// アカウントに保存されたロケールで描画する。通知は現在のリクエストの言語に紐づくのでは
// なく、アカウントの所有者に宛てるものだからである。
func (uc *VerifyEmailChangeUsecase) notifyOldAddress(ctx context.Context, oldEmail, newEmail, locale string) {
	if err := uc.dispatcher.EnqueueEmailChangeNotification(ctx, oldEmail, newEmail, locale); err != nil {
		slog.ErrorContext(ctx, "メールアドレス変更通知メールのジョブ投入に失敗", "error", err, "email", oldEmail)
	}
}

// verify compares the submitted code against the active confirmation and, in one
// transaction, either increments its failed-attempt count (wrong code) or stamps
// it succeeded and applies the new address to users.email (correct code). A wrong
// code commits the increment and returns a form-wide ValidationError whose message
// matches the validator's not-found/expired message, so the form never
// distinguishes a wrong code from a missing confirmation (enumeration protection).
// On a correct code the stamp and the email update share the transaction, so a
// failure of either rolls both back. A UNIQUE violation on the update means the new
// address was taken between the request-time check and now; it is surfaced as a
// form-wide ValidationError (not a 500) so the user can retry with another address.
//
// [Ja] verify は送信されたコードをアクティブな確認と照合し、1 トランザクションで、失敗
// 試行回数をインクリメントする (誤ったコード) か、確認を成功済みとして打刻し新しいアドレスを
// users.email に適用する (正しいコード) かのいずれかを行います。誤ったコードはインクリメントを
// コミットし、validator の未検出 / 期限切れメッセージと一致するフォーム全体の ValidationError を
// 返すため、フォームは誤ったコードと確認の不在を区別しません (列挙攻撃対策)。正しいコードでは
// 打刻とメール更新がトランザクションを共有するため、どちらかの失敗は両方をロールバックします。
// 更新時の UNIQUE 違反は、申請時のチェックから現在までの間に新しいアドレスが取得されたことを
// 意味し、フォーム全体の ValidationError (500 ではなく) として表面化させ、ユーザーが別の
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

	// UserID is non-nil for an email-change confirmation (it is issued by a
	// signed-in user), and Email is the new address to switch to.
	//
	// [Ja] UserID はメール変更の確認では非 nil であり (サインイン済みユーザーが発行する)、
	// Email は切り替え先の新しいアドレスである。
	if err := userRepo.UpdateEmail(ctx, *confirmation.UserID, confirmation.Email); err != nil {
		// The email-change apply hits the UNIQUE constraint when the new address is
		// claimed by another account between the request-time uniqueness check and
		// this update, and that race is turned into a user-fixable validation error
		// rather than a 500.
		//
		// [Ja] メール変更の適用は、申請時の一意性チェックからこの更新までの間に新しい
		// アドレスが別アカウントに取得されると UNIQUE 制約に当たる。その競合を 500 では
		// なくユーザーが修正できるバリデーションエラーに変換する。
		if repository.IsUniqueViolation(err) {
			// The address was claimed by another account after the request-time
			// check. Return without committing so the defer rolls back both the
			// stamp and the update, leaving the confirmation active; the user is
			// prompted to retry with a different address.
			//
			// [Ja] 申請時のチェックの後に別アカウントがアドレスを取得した。コミット
			// せずに返し、defer が打刻と更新の両方をロールバックして確認を active の
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
