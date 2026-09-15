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

// DeleteAccountUsecaseはユーザー自身によるアカウント退会を統括します。現在の
// パスワードを再確認し、1トランザクションでユーザーを論理削除し (deleted_atを打つ)、
// 解放されたemail / atnameを匿名化し、そのユーザーの全セッションとロールの割当を削除
// します。行とそのCASCADEする子データのより重い物理削除は後続の定期パージジョブに委ね
// ます。本ステップはアカウントを即座に無効化し、一意な識別子を解放するだけです。
//
// 最後の管理者の退会は拒否します。去るアカウントは自身のロールも連れて行くため、
// コミュニティが管理画面を開ける人を1人も持たない状態になるためです。
type DeleteAccountUsecase struct {
	writer          *sql.DB
	validator       *validator.SettingsWithdrawalDeleteValidator
	userRepo        *repository.UserRepository
	userSessionRepo *repository.UserSessionRepository
	roleRepo        *repository.RoleRepository
	userRoleRepo    *repository.UserRoleRepository
}

// NewDeleteAccountUsecaseは書き込み用プール・validator・読み書きに使うリポジトリ
// からDeleteAccountUsecaseを構築します。
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

// DeleteAccountInputはExecuteの入力です。UserIDは退会を申請するサインイン済み
// ユーザー、CurrentPasswordは申請を再認証するために送信されたフォーム値です。
type DeleteAccountInput struct {
	UserID          model.UserID
	CurrentPassword string
}

// Executeは現在のパスワードを検証してからアカウントを退会させます。バリデーションを
// 先に走らせるため、誤った / 未入力の現在のパスワードでは行に触れず
// *model.ValidationErrorを返します。匿名化したemailとatnameはトランザクションの前に
// 計算し (ユーザーidの純粋な関数のため)、adminロールもそこで引きます。このロールは
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
		return fmt.Errorf("adminロールの取得に失敗: %w", err)
	}

	return uc.deleteAccount(ctx, input.UserID, model.AnonymizedEmail(input.UserID), model.AnonymizedAtname(input.UserID), adminRole)
}

// deleteAccountはユーザーの論理削除・匿名化と、全セッション・全ロール割当の削除を
// 1トランザクションで行い、アカウントが中途半端に退会した状態を残さないようにします
// (users行の更新と2種類の子データの消去がすべて成るか、どれも成らないか)。この複数の
// 永続化ステップがあるため、本処理をExecute (純粋なオーケストレーションに徹する) から
// 切り出しています。
//
// そのアカウントが最後の管理者かどうかは同じトランザクションの中で読みます。拒否を判断した
// 数が、それが許した退会より前に変わらないようにするためです。同時に届いた剥奪は書き込み
// ロックを先に取るか、その解放を待つため、両者が「もう1人の管理者」を見つけながら、その
// 1人を片方しか残さない、という結果にはなりません。
//
// マイグレーションがadminロールを作っていないコミュニティには守るべき管理者がいないため、
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

// verifyWithdrawalKeepsAnAdminは、去ろうとしているアカウントが残る唯一の管理者で
// あるときに退会を拒否します。
//
// 拒否を *model.AppErrorではなくフォーム全体の *model.ValidationErrorにするのは、その人が
// 退会フォームを見ているためです。その人と退会の間に立っているのはコミュニティの状態であり、
// 先に別の人を管理者にすることで自ら変えられます。フィールドのエラーでは指し示す先が
// ありません。拒否されたのは、その人が入力したどのフィールドでもないためです。
//
// 去ろうとしているアカウントは、removingLeavesNoAdminが数える有効な保持者に常に含まれます。
// それがこのヘルパーが対象に求めるものです。退会を行うのはサインインしている本人であり、
// 停止されたアカウントはセッションから解決されないためです。
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
