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

// DeleteAccountUsecase orchestrates a user's self-service account withdrawal: it
// re-checks the current password, then in one transaction soft-deletes the user
// (stamping deleted_at), anonymizes the freed email/atname, and deletes all of the
// user's sessions and role assignments. The heavier physical delete of the row and
// its cascading children is left to a later periodic purge job; this step just
// makes the account inert immediately and releases the unique identifiers.
//
// Withdrawal is refused for the last administrator, since an account that leaves
// takes its roles with it and the community would be left with nobody able to
// open the admin screens.
//
// [Ja] DeleteAccountUsecase はユーザー自身によるアカウント退会を統括します。現在の
// パスワードを再確認し、1 トランザクションでユーザーを論理削除し (deleted_at を打つ)、
// 解放された email / atname を匿名化し、そのユーザーの全セッションとロールの割当を削除
// します。行とその CASCADE する子データのより重い物理削除は後続の定期パージジョブに委ね
// ます。本ステップはアカウントを即座に無効化し、一意な識別子を解放するだけです。
//
// 最後の管理者の退会は拒否します。去るアカウントは自身のロールも連れて行くため、
// コミュニティが管理画面を開ける人を 1 人も持たない状態になるためです。
type DeleteAccountUsecase struct {
	writer          *sql.DB
	validator       *validator.SettingsWithdrawalDeleteValidator
	userRepo        *repository.UserRepository
	userSessionRepo *repository.UserSessionRepository
	roleRepo        *repository.RoleRepository
	userRoleRepo    *repository.UserRoleRepository
}

// NewDeleteAccountUsecase builds a DeleteAccountUsecase from the write pool, its
// validator, and the repositories it reads and persists through.
//
// [Ja] NewDeleteAccountUsecase は書き込み用プール・validator・読み書きに使うリポジトリ
// から DeleteAccountUsecase を構築します。
func NewDeleteAccountUsecase(
	writer *sql.DB,
	validator *validator.SettingsWithdrawalDeleteValidator,
	userRepo *repository.UserRepository,
	userSessionRepo *repository.UserSessionRepository,
	roleRepo *repository.RoleRepository,
	userRoleRepo *repository.UserRoleRepository,
) *DeleteAccountUsecase {
	return &DeleteAccountUsecase{
		writer:          writer,
		validator:       validator,
		userRepo:        userRepo,
		userSessionRepo: userSessionRepo,
		roleRepo:        roleRepo,
		userRoleRepo:    userRoleRepo,
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
// and the admin role is looked up there too, since a migration creates it and
// nothing changes it afterwards.
//
// [Ja] Execute は現在のパスワードを検証してからアカウントを退会させます。バリデーションを
// 先に走らせるため、誤った / 未入力の現在のパスワードでは行に触れず
// *model.ValidationError を返します。匿名化した email と atname はトランザクションの前に
// 計算し (ユーザー id の純粋な関数のため)、admin ロールもそこで引きます。このロールは
// マイグレーションが作るものであり、その後は何も変えないためです。
func (uc *DeleteAccountUsecase) Execute(ctx context.Context, input DeleteAccountInput) error {
	if err := uc.validator.Validate(ctx, validator.SettingsWithdrawalDeleteValidatorInput{
		UserID:          input.UserID,
		CurrentPassword: input.CurrentPassword,
	}); err != nil {
		return err
	}

	adminRole, err := uc.roleRepo.FindByName(ctx, model.RoleNameAdmin)
	if err != nil {
		return fmt.Errorf("admin ロールの取得に失敗: %w", err)
	}

	return uc.deleteAccount(ctx, input.UserID, model.AnonymizedEmail(input.UserID), model.AnonymizedAtname(input.UserID), adminRole)
}

// deleteAccount soft-deletes and anonymizes the user and deletes all of their
// sessions and role assignments in one transaction, so the account is never left
// half-withdrawn: either the users row is updated and both sets of children are
// gone, or none of it happened. The several persistence steps are why this is
// split out of Execute (which stays pure orchestration).
//
// Whether the account is the last administrator is read inside the same
// transaction, so that the number the refusal is decided against cannot change
// before the withdrawal it admits. A revoke arriving at the same time takes the
// write lock first or waits for it, so the two cannot both find a second
// administrator that only one of them leaves behind.
//
// A community whose admin role a migration has not created has no administrator
// to protect, so the check is skipped rather than refusing every withdrawal.
//
// [Ja] deleteAccount はユーザーの論理削除・匿名化と、全セッション・全ロール割当の削除を
// 1 トランザクションで行い、アカウントが中途半端に退会した状態を残さないようにします
// (users 行の更新と 2 種類の子データの消去がすべて成るか、どれも成らないか)。この複数の
// 永続化ステップがあるため、本処理を Execute (純粋なオーケストレーションに徹する) から
// 切り出しています。
//
// そのアカウントが最後の管理者かどうかは同じトランザクションの中で読みます。拒否を判断した
// 数が、それが許した退会より前に変わらないようにするためです。同時に届いた剥奪は書き込み
// ロックを先に取るか、その解放を待つため、両者が「もう 1 人の管理者」を見つけながら、その
// 1 人を片方しか残さない、という結果にはなりません。
//
// マイグレーションが admin ロールを作っていないコミュニティには守るべき管理者がいないため、
// すべての退会を拒否するのではなく検査を飛ばします。
func (uc *DeleteAccountUsecase) deleteAccount(
	ctx context.Context,
	userID model.UserID,
	anonEmail, anonAtname string,
	adminRole *model.Role,
) error {
	tx, err := uc.writer.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("トランザクションの開始に失敗: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	userRepo := uc.userRepo.WithTx(tx)
	userSessionRepo := uc.userSessionRepo.WithTx(tx)
	userRoleRepo := uc.userRoleRepo.WithTx(tx)

	if adminRole != nil {
		if err := verifyWithdrawalKeepsAnAdmin(ctx, userRoleRepo, adminRole.ID, userID); err != nil {
			return err
		}
	}

	if err := userRepo.SoftDeleteAndAnonymize(ctx, userID, anonEmail, anonAtname); err != nil {
		return fmt.Errorf("ユーザーの論理削除・匿名化に失敗: %w", err)
	}
	if err := userSessionRepo.DeleteByUserID(ctx, userID); err != nil {
		return fmt.Errorf("ユーザーセッションの削除に失敗: %w", err)
	}
	if err := userRoleRepo.DeleteByUserID(ctx, userID); err != nil {
		return fmt.Errorf("ロールの割当の削除に失敗: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("トランザクションのコミットに失敗: %w", err)
	}
	return nil
}

// verifyWithdrawalKeepsAnAdmin refuses the withdrawal when the account leaving is
// the only administrator left.
//
// The refusal is a form-wide *model.ValidationError rather than an *model.AppError,
// because the person is looking at the withdrawal form: what stands between them
// and leaving is a state of the community they can change, by making someone else
// an administrator first. A field error would have nothing to point at, since no
// field they filled in is what was refused.
//
// [Ja] verifyWithdrawalKeepsAnAdmin は、去ろうとしているアカウントが残る唯一の管理者で
// あるときに退会を拒否します。
//
// 拒否を *model.AppError ではなくフォーム全体の *model.ValidationError にするのは、その人が
// 退会フォームを見ているためです。その人と退会の間に立っているのはコミュニティの状態であり、
// 先に別の人を管理者にすることで自ら変えられます。フィールドのエラーでは指し示す先が
// ありません。拒否されたのは、その人が入力したどのフィールドでもないためです。
func verifyWithdrawalKeepsAnAdmin(
	ctx context.Context,
	userRoleRepo *repository.UserRoleRepository,
	adminRoleID model.RoleID,
	userID model.UserID,
) error {
	holds, err := holdsRole(ctx, userRoleRepo, userID, adminRoleID)
	if err != nil {
		return err
	}
	if !holds {
		return nil
	}

	leavesNoAdmin, err := removingLeavesNoAdmin(ctx, userRoleRepo, adminRoleID)
	if err != nil {
		return err
	}
	if !leavesNoAdmin {
		return nil
	}

	ve := model.NewValidationError()
	ve.AddGlobal(i18n.T(ctx, "validation_withdrawal_last_admin"))
	return ve
}
