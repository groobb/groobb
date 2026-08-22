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

// anonymizedEmail derives the placeholder email a withdrawn account's email is
// overwritten with. It embeds the user id so the value is distinct per account,
// freeing the original address for re-registration.
//
// The reserved .invalid TLD (RFC 2606) is what makes the value unreachable: both
// sign-up and an email change require a confirmation code delivered to the
// address, and .invalid can never receive one, so no account can hold this value
// and the overwrite cannot lose the users.email UNIQUE constraint to a live row.
//
// [Ja] anonymizedEmail は退会済みアカウントの email を上書きする代替 email を導出します。
// ユーザー id を埋め込むことで値をアカウントごとに別のものにし、元のアドレスを再登録用に
// 解放します。
//
// 値を到達不能にしているのは予約 TLD の .invalid (RFC 2606) です。サインアップもメール
// アドレス変更も、アドレスへ配送された確認コードを要求しますが、.invalid はそれを受け取れ
// ません。そのためこの値を保持できるアカウントは存在せず、上書きが実在の行との間で
// users.email の UNIQUE 制約に負けることがありません。
func anonymizedEmail(userID model.UserID) string {
	return fmt.Sprintf("deleted-%s@deleted.invalid", userID.String())
}

// anonymizedAtname derives the placeholder atname a withdrawn account's atname is
// overwritten with. It embeds the user id so the value is distinct per account,
// freeing the original atname for reuse.
//
// The hyphen is what makes the value unreachable: it is outside the atname
// character set of ASCII letters, digits, and underscore, so no account can ever
// hold this value and the overwrite cannot lose the users.atname UNIQUE
// constraint to a live row. A separator inside that character set would not be
// enough, because an id in decimal is short enough to spell a tombstone that
// passes the account form and squats the value the owner's withdrawal needs.
// users.atname is NOCASE-collated TEXT with no length bound or format check, so
// the column accepts the hyphen; this tombstone value is never re-validated or
// shown as a handle.
//
// [Ja] anonymizedAtname は退会済みアカウントの atname を上書きする代替 atname を導出
// します。ユーザー id を埋め込むことで値をアカウントごとに別のものにし、元の atname を
// 再利用向けに解放します。
//
// 値を到達不能にしているのはハイフンです。ハイフンは atname の文字集合である ASCII
// 英数字 / アンダースコアの外にあるため、この値を保持できるアカウントは存在せず、上書きが
// 実在の行との間で users.atname の UNIQUE 制約に負けることがありません。区切りを文字集合
// 内の文字にするとこれは成り立ちません。10 進表記の id は短く、アカウントフォームを通る
// 墓標値を綴れてしまうため、退会に必要な値を先取りされうるからです。users.atname は長さ
// 上限も形式チェックも無い NOCASE 照合の TEXT のため、カラム自体はハイフンを受け付けます。
// この墓標値は再検証もハンドルとしての表示もされません。
func anonymizedAtname(userID model.UserID) string {
	return "deleted-" + userID.String()
}
