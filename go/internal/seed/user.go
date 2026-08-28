package seed

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
)

// seedRole is the logical name by which a generator asks for an account. A
// generator names the account that opens a thread or the one that answers it,
// never the first or the second account, so that an account added to the roster
// leaves the generators that do not need its role untouched.
//
// [Ja] seedRole は、生成器がアカウントを求めるときに使う論理名です。生成器が名指し
// するのは「1 人目」「2 人目」ではなく、スレッドを立てるアカウントや、それに答える
// アカウントです。名簿にアカウントを 1 つ足したとき、その役割を必要としない生成器が
// 変わらないようにするためです。
type seedRole string

const (
	// roleStarter opens the threads a board lists. It is the account the browser
	// verification signs in as unless another role is asked for, because the
	// screens reached from a thread it wrote are the ones most often looked at.
	//
	// [Ja] roleStarter は、掲示板に並ぶスレッドを立てるアカウントです。別の役割を
	// 指定しない限りブラウザ確認がサインインするのはこのアカウントです。ここが書いた
	// スレッドから辿れる画面が、もっともよく見られる画面であるためです。
	roleStarter seedRole = "starter"

	// roleReplier answers threads it did not open. Two accounts are what make a
	// thread read as a conversation, which is what a reply reference (>>N) and
	// the back reference under the post it points at are there to show.
	//
	// [Ja] roleReplier は、自分が立てたのではないスレッドに返信するアカウントです。
	// スレッドが会話として読めるようになるのはアカウントが 2 つあるからであり、レス参照
	// (>>N) と、その指し先の投稿に付く逆参照が見せるのはその会話です。
	roleReplier seedRole = "replier"

	// roleWithdrawn is the account a post is looked at without its author: a
	// withdrawal leaves what was written in place and takes only the name off
	// it, so a screen has to be checked with an author that is no longer there.
	//
	// [Ja] roleWithdrawn は、投稿を作者抜きで眺めるためのアカウントです。退会は書かれた
	// ものをその場に残し、名前だけを外すため、作者がもういない状態でも画面を確認する
	// 必要があります。
	roleWithdrawn seedRole = "withdrawn"
)

// allSeedRoles lists every role a generator names. The roster has to hold one
// account for each of them, which is what lets a generator ask for a role and
// get an account back.
//
// [Ja] allSeedRoles は生成器が名指しする役割の一覧です。名簿はこのそれぞれに 1 件ずつ
// アカウントを持つ必要があり、それによって生成器は役割を求めてアカウントを受け取れます。
var allSeedRoles = []seedRole{roleStarter, roleReplier, roleWithdrawn}

// signInSeedRoles lists the roles whose accounts remain active after a seeding
// run finishes. The withdrawn role is required while content is generated, but
// that account is anonymized and made unable to sign in before the run returns.
//
// [Ja] signInSeedRoles は、シード実行の完了後もアカウントが有効な役割の一覧です。
// withdrawn はコンテンツ生成中には必要ですが、そのアカウントは実行が返る前に匿名化され、
// サインインできない状態になります。
var signInSeedRoles = []seedRole{roleStarter, roleReplier}

// SignInRoles lists the roles FindCredentials answers, as the names written in
// the roster. The usage of groobb devcreds is built from this, so that the line
// names the roles the subcommand takes instead of leaving them to be found by
// asking for one it does not.
//
// [Ja] SignInRoles は、FindCredentials が応じる役割を、名簿に書く名前として挙げます。
// groobb devcreds の usage をここから組み立てることで、その 1 行が、受け付けない役割を
// 指定して探し当てるのではなく、受け付ける役割そのものを挙げられるようになります。
func SignInRoles() []string {
	roles := make([]string, 0, len(signInSeedRoles))
	for _, role := range signInSeedRoles {
		roles = append(roles, string(role))
	}

	return roles
}

// seedUserTimeZone is the time zone the seeded accounts carry. Groobb has no
// screen that changes it, so every account takes the same value the application
// assigns at sign-up.
//
// [Ja] seedUserTimeZone はシードが作るアカウントが持つタイムゾーンです。Groobb には
// これを変更する画面が無いため、どのアカウントもアプリケーションがサインアップ時に
// 割り当てるのと同じ値を取ります。
const seedUserTimeZone = "Asia/Tokyo"

// seededUsers holds the accounts a run created, for the generators that come
// after it.
//
// [Ja] seededUsers は実行が作成したアカウントを、後続の生成器のために保持します。
type seededUsers struct {
	byRole map[seedRole]*model.User
}

// user returns the account created for the role, or nil when the run created
// none for it.
//
// [Ja] user は、その役割で作成したアカウントを返します。実行がその役割のアカウントを
// 作っていない場合は nil を返します。
func (u *seededUsers) user(role seedRole) *model.User {
	return u.byRole[role]
}

// generateUsers creates the accounts the roster names, which are the accounts
// the browser verification signs in as.
//
// [Ja] generateUsers は名簿が挙げるアカウントを作成します。それはブラウザ確認で
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

// createUser creates a user together with the password digest that lets it sign
// in.
//
// The rows go through the repositories the application itself creates an account
// with, rather than through statements of the seed's own, so that a seeded
// account is the same shape as a signed-up one. Seeding is not a reason to grow
// the Infrastructure layer with code only the seed calls, but it is no reason to
// avoid what is already there either.
//
// [Ja] createUser はユーザーと、サインインに使うパスワードダイジェストを作成します。
//
// 行はシード専用の文ではなく、アプリケーション自身がアカウントを作成するのに使うリポジトリ
// を通します。シードが作るアカウントを、サインアップしたアカウントと同じ形にするためです。
// シードのために、シードだけが呼ぶコードを Infrastructure 層へ増やすことはしませんが、
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
		Locale:   i18n.DefaultLang,
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

// generateWithdrawal withdraws the account whose posts are looked at without an
// author. It runs after the conversations have been written, because a
// withdrawal is something that happens to an account that has already posted:
// the writing stays where it is, and the account's name comes off it.
//
// The withdrawal is the one the application performs, down to the values the
// freed email and atname are overwritten with. A seed that invented its own
// tombstone would put the account in a state no withdrawal produces, which is
// the one thing a screen showing a withdrawn author must not be checked
// against.
//
// [Ja] generateWithdrawal は、投稿を作者抜きで眺めるためのアカウントを退会させます。
// これが会話を書き終えた後に走るのは、退会が、既に投稿したアカウントに起きることだから
// です。書かれたものはその場に残り、そこからアカウントの名前が外れます。
//
// 退会はアプリケーションが行うものそのもので、解放された email と atname を上書きする値も
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
