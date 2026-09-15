package seed

import (
	"fmt"
	"slices"
)

// Credentialsは、シードが作成したアカウント1件がサインインに使う値です。
type Credentials struct {
	Email    string
	Password string
}

// FindCredentialsは、名簿がroleのアカウントへ与える資格情報を返します。応じるのは
// SignInRolesが挙げる役割だけです。名簿には存在していても、完了した実行が無効化した役割は、
// 資格情報ではなくエラーになります。
//
// ブラウザ確認が自前の環境変数を読むのではなくここへ尋ねるのは、サインインするアカウントを、
// シードが作成したアカウントそのものにするためです。供給元が2つあると片方だけを変えられて
// しまい、どちらか一方が単体で間違っているわけでもないまま、シードの後のサインインが通らなく
// なります。
func FindCredentials(role string) (*Credentials, error) {
	return findCredentials(rosterPath, role)
}

// findCredentialsはpathの名簿を読み、roleの資格情報を返します。roleは
// signInSeedRolesのいずれかである必要があります。パスを受け取るのは、テストが自前の名簿を
// 指せるようにするためで、呼び出し側には実行が読むのと同じ1つのファイルが渡ります。
func findCredentials(path string, role string) (*Credentials, error) {
	file, err := loadRosterFile(path)
	if err != nil {
		return nil, err
	}

	// 尋ねられた1件だけでなく、名簿全体を検査します。これはシードが読むファイルで
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

	// 上の検査は、サインイン用の役割が生成器の役割でもあるため、そのすべてを名簿に
	// 要求します。ここへ到達するのは2つの一覧が一致しなくなった場合であるため、未知の役割と
	// するのではなく、崩れた不変条件をエラーに示します。
	return nil, fmt.Errorf("the development account roster %s has no account with the required sign-in role %s", path, requestedRole)
}
