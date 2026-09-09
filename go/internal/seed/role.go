package seed

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
)

// generateUserRoles gives the admin account the built-in admin role, so that the
// admin screens can be opened right after a run.
//
// It runs after the accounts have been created, because an assignment belongs to
// a user, and before the content, because what an account holds is part of who
// it is rather than something it produces.
//
// The role itself is not created here. Roles are what a migration writes and the
// cleanup preserves, so a run that created one would be writing a row the
// application cannot make again once it is gone.
//
// [Ja] generateUserRoles は、管理者用のアカウントへ組み込みの admin ロールを与えます。実行の
// 直後から管理画面を開けるようにするためです。
//
// これがアカウントの作成後に走るのは、割当がユーザーに属するためであり、コンテンツより先に
// 走るのは、アカウントが何を持つかが、そのアカウントが何を生み出したかではなく、それが誰で
// あるかの一部だからです。
//
// ロールそのものはここでは作りません。ロールはマイグレーションが書き、クリーンアップが
// 残すものであり、実行がそれを作るなら、それは失われたらアプリケーションには作り直せない行を
// 書くことになります。
func (r *Runner) generateUserRoles(ctx context.Context, tx *sql.Tx, st *state) error {
	bar := newProgress(r.out, "user roles", 1)
	defer bar.finish()

	user := st.users.user(roleAdmin)
	if user == nil {
		return fmt.Errorf("no account was created for the role %s", roleAdmin)
	}

	role, err := repository.NewRoleRepository(r.db).WithTx(tx).FindByName(ctx, model.RoleNameAdmin)
	if err != nil {
		return fmt.Errorf("failed to find the %s role: %w", model.RoleNameAdmin, err)
	}
	if role == nil {
		return fmt.Errorf("the database holds no %s role; the migrations that create it have not been applied", model.RoleNameAdmin)
	}

	userRoleRepo := repository.NewUserRoleRepository(r.db).WithTx(tx)
	if _, err := userRoleRepo.Create(ctx, repository.CreateUserRoleInput{
		UserID: user.ID,
		RoleID: role.ID,
	}); err != nil {
		return fmt.Errorf("failed to give the account %s the %s role: %w", user.Atname, model.RoleNameAdmin, err)
	}

	bar.advance()

	return nil
}
