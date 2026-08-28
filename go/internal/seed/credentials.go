package seed

import (
	"fmt"
	"slices"
)

// Credentials are what one seeded account signs in with.
//
// [Ja] Credentials は、シードが作成したアカウント 1 件がサインインに使う値です。
type Credentials struct {
	Email    string
	Password string
}

// FindCredentials returns the credentials the roster gives the account with
// role. Only the roles SignInRoles lists are answered: a role the roster holds
// but a completed run has disabled is an error rather than a credential.
//
// The browser verification asks for them here rather than reading an
// environment of its own, so that the account it signs in as is the account the
// seed created. Two sources let one of them be changed alone, and the sign-in
// that follows a run then stops working without either of them being wrong on
// its own.
//
// [Ja] FindCredentials は、名簿が role のアカウントへ与える資格情報を返します。応じるのは
// SignInRoles が挙げる役割だけです。名簿には存在していても、完了した実行が無効化した役割は、
// 資格情報ではなくエラーになります。
//
// ブラウザ確認が自前の環境変数を読むのではなくここへ尋ねるのは、サインインするアカウントを、
// シードが作成したアカウントそのものにするためです。供給元が 2 つあると片方だけを変えられて
// しまい、どちらか一方が単体で間違っているわけでもないまま、シードの後のサインインが通らなく
// なります。
func FindCredentials(role string) (*Credentials, error) {
	return findCredentials(rosterPath, role)
}

// findCredentials reads the roster at path and returns the credentials for
// role, which has to be one of signInSeedRoles. It takes the path so that a
// test can point it at a roster of its own, while callers get the one file a
// run reads.
//
// [Ja] findCredentials は path の名簿を読み、role の資格情報を返します。role は
// signInSeedRoles のいずれかである必要があります。パスを受け取るのは、テストが自前の名簿を
// 指せるようにするためで、呼び出し側には実行が読むのと同じ 1 つのファイルが渡ります。
func findCredentials(path string, role string) (*Credentials, error) {
	file, err := loadRosterFile(path)
	if err != nil {
		return nil, err
	}

	// The whole roster is checked over, not only the entry that was asked for.
	// This is the file the seed reads, so credentials taken out of a roster the
	// seed would refuse belong to an account the database does not hold, and
	// signing in with them fails at the form instead of here, where what is
	// wrong with the file can still be said.
	//
	// [Ja] 尋ねられた 1 件だけでなく、名簿全体を検査します。これはシードが読むファイルで
	// あり、シードが拒否する名簿から取り出した資格情報は、データベースに存在しないアカウント
	// のものです。それを使ったサインインは、ファイルの何が問題なのかを告げられるここではなく、
	// フォームで失敗することになります。
	users, err := file.validate()
	if err != nil {
		return nil, fmt.Errorf("the development account roster %s: %w", path, err)
	}

	requestedRole := seedRole(role)
	if !slices.Contains(signInSeedRoles, requestedRole) {
		return nil, fmt.Errorf(
			"the role %q does not name an account that can sign in after seeding; the sign-in roles are %s",
			role,
			joinSeedRoles(signInSeedRoles),
		)
	}

	for _, user := range users {
		if user.role == requestedRole {
			return &Credentials{Email: user.email, Password: file.Password}, nil
		}
	}

	// Validation above requires every sign-in role because those roles are also
	// generator roles. Reaching this point would mean the two lists no longer
	// agree, so the error names the invariant that was broken rather than calling
	// the role unknown.
	//
	// [Ja] 上の検査は、サインイン用の役割が生成器の役割でもあるため、そのすべてを名簿に
	// 要求します。ここへ到達するのは 2 つの一覧が一致しなくなった場合であるため、未知の役割と
	// するのではなく、崩れた不変条件をエラーに示します。
	return nil, fmt.Errorf("the development account roster %s has no account with the required sign-in role %s", path, requestedRole)
}
