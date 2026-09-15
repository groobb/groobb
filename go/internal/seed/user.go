package seed

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
)

// seedRoleは、生成器がアカウントを求めるときに使う論理名です。生成器が名指し
// するのは「1人目」「2人目」ではなく、スレッドを立てるアカウントや、それに答える
// アカウントです。名簿にアカウントを1つ足したとき、その役割を必要としない生成器が
// 変わらないようにするためです。
type seedRole string

const (
	// roleStarterは、掲示板に並ぶスレッドを立てるアカウントです。別の役割を
	// 指定しない限りブラウザ確認がサインインするのはこのアカウントです。ここが書いた
	// スレッドから辿れる画面が、もっともよく見られる画面であるためです。
	roleStarter seedRole = "starter"

	// roleReplierは、自分が立てたのではないスレッドに返信するアカウントです。
	// スレッドが会話として読めるようになるのはアカウントが2つあるからであり、レス参照
	// (>>N) と、その指し先の投稿に付く逆参照が見せるのはその会話です。
	roleReplier seedRole = "replier"

	// roleAdminは組み込みのadminロールを持つアカウントです。サブコマンドで先に
	// 誰かを任命しなくても管理画面を開けるようにするためです。roleStarterにそのロールを
	// 与えるのではなく専用のアカウントにしているのは、他のアカウントで見る画面が、管理者で
	// ない人として眺められるものであるためです。スレッドを書いた本人がコミュニティの管理も
	// するアカウントにすると、そこから確認するどの画面にも管理画面へのリンクが入ります。
	roleAdmin seedRole = "admin"

	// roleWithdrawnは、投稿を作者抜きで眺めるためのアカウントです。退会は書かれた
	// ものをその場に残し、名前だけを外すため、作者がもういない状態でも画面を確認する
	// 必要があります。
	roleWithdrawn seedRole = "withdrawn"
)

// allSeedRolesは生成器が名指しする役割の一覧です。名簿はこのそれぞれに1件ずつ
// アカウントを持つ必要があり、それによって生成器は役割を求めてアカウントを受け取れます。
var allSeedRoles = []seedRole{roleStarter, roleReplier, roleAdmin, roleWithdrawn}

// signInSeedRolesは、シード実行の完了後もアカウントが有効な役割の一覧です。
// withdrawnはコンテンツ生成中には必要ですが、そのアカウントは実行が返る前に匿名化され、
// サインインできない状態になります。
var signInSeedRoles = []seedRole{roleStarter, roleReplier, roleAdmin}

// SignInRolesは、FindCredentialsが応じる役割を、名簿に書く名前として挙げます。
// groobb devcredsのusageをここから組み立てることで、その1行が、受け付けない役割を
// 指定して探し当てるのではなく、受け付ける役割そのものを挙げられるようになります。
func SignInRoles() []string {
	roles := make([]string, 0, len(signInSeedRoles))
	for _, role := range signInSeedRoles {
		roles = append(roles, string(role))
	}

	return roles
}

// seedUserTimeZoneはシードが作るアカウントが持つタイムゾーンです。Groobbには
// これを変更する画面が無いため、どのアカウントもアプリケーションがサインアップ時に
// 割り当てるのと同じ値を取ります。
const seedUserTimeZone = "Asia/Tokyo"

// seededUsersは実行が作成したアカウントを、後続の生成器のために保持します。
type seededUsers struct {
	byRole map[seedRole]*model.User
}

// userは、その役割で作成したアカウントを返します。実行がその役割のアカウントを
// 作っていない場合はnilを返します。
func (u *seededUsers) user(role seedRole) *model.User {
	return u.byRole[role]
}

// generateUsersは名簿が挙げるアカウントを作成します。それはブラウザ確認で
// サインインするアカウントです。
func (r *Runner) generateUsers(ctx context.Context, tx *sql.Tx, st *state) error {
	bar := newProgress(r.out, "users", len(st.roster.users))
	defer bar.finish()

	userRepo := repository.NewUserRepository(r.db).WithTx(tx)
	userPasswordRepo := repository.NewUserPasswordRepository(r.db).WithTx(tx)

	users := &seededUsers{byRole: make(map[seedRole]*model.User, len(st.roster.users))}

	for _, account := range st.roster.users {
		user, err := createUser(ctx, userRepo, userPasswordRepo, account, st.roster.passwordDigest)
		if err != nil {
			return err
		}

		users.byRole[account.role] = user
		bar.advance()
	}

	st.users = users

	return nil
}

// createUserはユーザーと、サインインに使うパスワードダイジェストを作成します。
//
// 行はシード専用の文ではなく、アプリケーション自身がアカウントを作成するのに使うリポジトリ
// を通します。シードが作るアカウントを、サインアップしたアカウントと同じ形にするためです。
// シードのために、シードだけが呼ぶコードをInfrastructure層へ増やすことはしませんが、
// 既にあるものを避ける理由も同じくありません。
func createUser(
	ctx context.Context,
	userRepo *repository.UserRepository,
	userPasswordRepo *repository.UserPasswordRepository,
	account rosterUser,
	passwordDigest string,
) (*model.User, error) {
	user, err := userRepo.Create(ctx, repository.CreateUserInput{
		Email:    account.email,
		Atname:   account.atname,
		Locale:   model.DefaultLocale,
		TimeZone: seedUserTimeZone,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create the user %s: %w", account.atname, err)
	}

	if _, err := userPasswordRepo.Create(ctx, repository.CreateUserPasswordInput{
		UserID:         user.ID,
		PasswordDigest: passwordDigest,
	}); err != nil {
		return nil, fmt.Errorf("failed to create the password of the user %s: %w", account.atname, err)
	}

	return user, nil
}

// generateWithdrawalは、投稿を作者抜きで眺めるためのアカウントを退会させます。
// これが会話を書き終えた後に走るのは、退会が、既に投稿したアカウントに起きることだから
// です。書かれたものはその場に残り、そこからアカウントの名前が外れます。
//
// 退会はアプリケーションが行うものそのもので、解放されたemailとatnameを上書きする値も
// 同じです。シードが独自の墓標値を作れば、アカウントはどの退会も生み出さない状態に置かれる
// ことになります。退会したアカウントを表示する画面を、それに照らして確かめてはなりません。
func (r *Runner) generateWithdrawal(ctx context.Context, tx *sql.Tx, st *state) error {
	bar := newProgress(r.out, "withdrawal", 1)
	defer bar.finish()

	user := st.users.user(roleWithdrawn)
	if user == nil {
		return fmt.Errorf("no account was created for the role %s", roleWithdrawn)
	}

	userRepo := repository.NewUserRepository(r.db).WithTx(tx)
	if err := userRepo.SoftDeleteAndAnonymize(ctx, user.ID, model.AnonymizedEmail(user.ID), model.AnonymizedAtname(user.ID)); err != nil {
		return fmt.Errorf("failed to withdraw the account %s: %w", user.Atname, err)
	}

	bar.advance()

	return nil
}
