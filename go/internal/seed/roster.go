package seed

import (
	"errors"
	"fmt"
	"io/fs"
	"net/mail"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/groobb/groobb/go/internal/auth"
	"github.com/groobb/groobb/go/internal/validator"
)

// rosterPathは実行がアカウントを読み込むファイル、rosterExamplePathはその代わりに
// コミットしている見本です。どちらも実行を開始したディレクトリからの相対パスであり、
// それはGoモジュールのルートになります (go/Makefileのseedターゲットを参照)。
//
// 名簿は個人のメールアドレスを持つためバージョン管理には入れず、代わりに見本を
// コミットしています。見本は誰かを名指しすることなく、開発環境にどんなアカウントがいて、
// それぞれが何を確認するためにいるのかを説明します。
const (
	rosterPath        = "seed-users.toml"
	rosterExamplePath = "seed-users.example.toml"
)

// rosterFileは、ファイルに書かれたままの名簿です。
type rosterFile struct {
	// Passwordは全アカウントで共通です。シードは開発環境以外での実行を拒否し、
	// devサイト自体もBasic認証の内側にあるため、アカウントごとに別のパスワードを
	// 持たせても得るものが無く、アカウントの数だけパスワード管理の項目が増えます。
	Password string           `toml:"password"`
	Users    []rosterUserFile `toml:"users"`
}

// rosterUserFileは、ファイルに書かれたままの [[users]] 1件です。
type rosterUserFile struct {
	Role   string `toml:"role"`
	Atname string `toml:"atname"`
	Email  string `toml:"email"`
	Note   string `toml:"note"`
}

// rosterUserは、名簿が挙げるアカウント1件です。
type rosterUser struct {
	role   seedRole
	atname string
	email  string
	// noteは、そのアカウントが何を見るためにいるのかを述べます。実行はこれを、
	// そのアカウントをサインインさせるアドレスと並べて報告します。開発者が、どのatnameが
	// 何を書いたかを覚えているかどうかではなく、何を見せるアカウントなのかで選べるように
	// するためです。
	note string
}

// userRosterは実行が作成するアカウントです。誰がいるのかはコードではなく設定と
// します。アドレスが個人のものであることと、アカウントが足されるのは既存のアカウントでは
// 取れない視点から画面を見るためであることによります。どのアカウントが何をするのかは
// コードに残り、コードはアカウントを役割で引きます。
type userRoster struct {
	// pathは名簿を読み込んだファイルです。実行はこれを、これから空にする
	// データベースと並べて報告します。実行が何を向いているのかを1行で読み取れるように
	// するためです。
	path string
	// passwordDigestは、アカウントが保存する形にした共通パスワードです。平文は
	// 読み込みの先へは持ち越しません。実行が書き込むのはダイジェストであり、名簿の
	// 読み込み時に一度ハッシュ化していることが、平文を落とせる理由になります。
	passwordDigest string
	users          []rosterUser
}

// loadUserRosterはpathから名簿を読みます。
//
// ファイルが無い場合は、見本へフォールバックせずエラーにします。見本が持つのは仮の
// アドレスであり、誰もメールを読まないアカウントでサインインする状態へ、気付かないまま
// 辿り着いてよいものではないためです。
func loadUserRoster(path string) (*userRoster, error) {
	file, err := loadRosterFile(path)
	if err != nil {
		return nil, err
	}

	roster, err := file.toUserRoster(path)
	if err != nil {
		return nil, fmt.Errorf("the development account roster %s: %w", path, err)
	}

	return roster, nil
}

// loadRosterFileはpathの名簿を、書かれたままの形で読みます。中身が何であるかの
// 検査は行いません。
func loadRosterFile(path string) (rosterFile, error) {
	var file rosterFile

	meta, err := toml.DecodeFile(path, &file)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return rosterFile{}, fmt.Errorf("the development account roster %s does not exist; copy %s to create it", path, rosterExamplePath)
		}

		return rosterFile{}, rosterFileError(path, err)
	}

	// デコーダが使わなかったキーは書き間違いです。その隣に書かれた値はアカウントへ
	// 届かず、実行はそのキーが与えるはずだったものを欠いたまま、そのアカウントを作りに
	// 行きます。
	if keys := meta.Undecoded(); len(keys) > 0 {
		return rosterFile{}, fmt.Errorf("the development account roster %s has keys that do not exist: %s", path, joinTOMLKeys(keys))
	}

	return file, nil
}

// toUserRosterは名簿を検査し、生成器が使う形にして返します。
func (f rosterFile) toUserRoster(path string) (*userRoster, error) {
	users, err := f.validate()
	if err != nil {
		return nil, err
	}

	// 名簿を読み込んでいる間に共通パスワードをハッシュ化します。bcryptが処理
	// できない入力をデータベースへ触る前に拒否できるだけでなく、各アカウントが準備済みの
	// 同じダイジェストを使えるため、一部のアカウントを書き込んだ後でハッシュ化の失敗が
	// 判明することも防げます。
	passwordDigest, err := auth.HashPassword(f.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	return &userRoster{
		path:           path,
		passwordDigest: passwordDigest,
		users:          users,
	}, nil
}

// validateは名簿を検査し、そこに書かれているアカウントを返します。
//
// パスワードダイジェストの手前で止まる点がtoUserRosterとの違いです。1件分の資格情報を
// ブラウザ確認へ渡すために名簿を読むときに要るのは書かれたままのパスワードであり、そこで
// ハッシュ化しても、かかる待ち時間に見合うものがないためです。
func (f rosterFile) validate() ([]rosterUser, error) {
	if f.Password == "" {
		return nil, errors.New("password is empty; write the sign-in password shared by every account")
	}
	if strings.ContainsAny(f.Password, "\r\n") {
		return nil, errors.New("password cannot contain CR or LF; write a password without a line break")
	}
	if len(f.Users) == 0 {
		return nil, errors.New("there is no [[users]] entry")
	}

	users := make([]rosterUser, 0, len(f.Users))
	roles := make(map[seedRole]bool, len(f.Users))
	atnames := make(map[string]bool, len(f.Users))
	emails := make(map[string]bool, len(f.Users))

	for i, entry := range f.Users {
		user, err := entry.toRosterUser()
		if err != nil {
			return nil, fmt.Errorf("[[users]] entry %d: %w", i+1, err)
		}

		// 役割は生成器がアカウントを名指しする名前、atnameは投稿が誰のものかを示す
		// ハンドル、メールアドレスはブラウザ確認がサインインに使う名前です。同じ値を
		// 2度書くと、共有した名前ではどちらか一方のアカウントへ辿り着けなくなります。
		//
		// どちらの列もNOCASE照合のため、大文字小文字だけが違う2つの値は、2件目の
		// アカウントが書き込まれるUNIQUE制約にとって同じ値です。ここでも同じ方法で
		// 比較することで、この衝突がデータベースを空にした後のINSERT失敗として表面化
		// することを防ぎます。
		atnameKey := strings.ToLower(user.atname)
		emailKey := strings.ToLower(user.email)
		if roles[user.role] {
			return nil, fmt.Errorf("there is more than one [[users]] entry with the role %s", user.role)
		}
		if atnames[atnameKey] {
			return nil, fmt.Errorf("there is more than one [[users]] entry with the atname %q (letter case does not tell two atnames apart)", user.atname)
		}
		if emails[emailKey] {
			return nil, fmt.Errorf("there is more than one [[users]] entry with the email %q (letter case does not tell two addresses apart)", user.email)
		}
		roles[user.role] = true
		atnames[atnameKey] = true
		emails[emailKey] = true

		users = append(users, user)
	}

	// 生成器が名指ししているのに名簿に無い役割は、それを必要とする生成器が走るまで
	// 表面化せず、それは実行がデータベースを空にした後になります。
	for _, role := range allSeedRoles {
		if !roles[role] {
			return nil, fmt.Errorf("there is no [[users]] entry with the role %s; the generators name this role, so one is required", role)
		}
	}

	return users, nil
}

// toRosterUserは1件分を検査し、生成器が使う形にして返します。
func (e rosterUserFile) toRosterUser() (rosterUser, error) {
	for _, field := range []struct {
		key   string
		value string
	}{
		{key: "role", value: e.Role},
		{key: "atname", value: e.Atname},
		{key: "email", value: e.Email},
		{key: "note", value: e.Note},
	} {
		if strings.TrimSpace(field.value) == "" {
			return rosterUser{}, fmt.Errorf("%s is empty", field.key)
		}
	}

	role := seedRole(e.Role)
	if !slices.Contains(allSeedRoles, role) {
		return rosterUser{}, fmt.Errorf("the role %q is not one the generators know; the roles are %s", e.Role, joinSeedRoles(allSeedRoles))
	}

	// atnameは、ここに置いた写しではなくアプリケーションがすべてのアカウントに
	// 課している規則で検査します。名簿が受理するatnameが、アカウントが実際に持てる
	// atnameであるようにするためです。名簿の読み込み時に検査することで、データベースを
	// 空にした後で初めて不正なアカウントが見つかることを防ぎます。
	if !validator.IsValidAtname(e.Atname) {
		return rosterUser{}, fmt.Errorf(
			"the atname %q may hold only ASCII letters, digits and underscores, and at most %d of them",
			e.Atname, validator.AtnameMaxLength,
		)
	}

	// 名簿が受理するメールアドレスは、そのアカウントをサインインさせるアドレスその
	// ものである必要があります。名簿が保存するのは解釈した結果ではなく書かれた文字列で
	// ある一方、前後の空白や表示名は解釈の過程で落ちます。どちらの書き方をしたアカウントも、
	// 同じ空白を落とすサインインフォームからは送信できないアドレスを持つことになります。
	// 書かれた文字列がアドレスそのものであることを求めることで、誰も辿り着けないアカウントを
	// 作ったまま実行が正常終了することを防ぎます。
	address, err := mail.ParseAddress(e.Email)
	if err != nil {
		return rosterUser{}, errors.New("email is not an email address")
	}
	if address.Name != "" || address.Address != e.Email {
		return rosterUser{}, fmt.Errorf(
			"email must hold the address alone (no display name, no surrounding whitespace); write it as %q if that is what was meant",
			address.Address,
		)
	}

	// noteは、照らし合わせる形式を自身では持たない唯一の必須文字列です。atnameは
	// 規則が空白を弾き、emailは書かれた文字列がアドレスそのものであることを求められますが、
	// 覚え書きは書かれたものが何であれ覚え書きになります。トリムすることで、ファイルに
	// 紛れ込んだ空白が、字下げされて見える報告行になることを防ぎます。
	return rosterUser{
		role:   role,
		atname: e.Atname,
		email:  e.Email,
		note:   strings.TrimSpace(e.Note),
	}, nil
}

// rosterFileErrorは、名簿の読み込み・デコードの失敗を、問題の位置は示しつつ
// そこにファイルが何と書いてあるかは繰り返さないエラーへ変換します。
//
// 構文エラーのメッセージはパーサーがつまずいたトークンを引用するため (引用符の無い
// パスワードに対する `expected value but found "hunter" instead` など)、ログへ出るこの
// エラーからはメッセージを落とし、位置と直前に解析したキーだけを残します。名簿は、
// すべてのアカウントがサインインに使うパスワードを持ちます。それ以外はそのまま通します。
// 残るデコードエラーが報告するのは値ではなく型です。
func rosterFileError(path string, err error) error {
	var parseErr toml.ParseError
	if !errors.As(err, &parseErr) {
		return fmt.Errorf("failed to read the development account roster %s: %w", path, err)
	}

	where := fmt.Sprintf("line %d", parseErr.Position.Line)
	if parseErr.LastKey != "" {
		where = fmt.Sprintf("%s (last key %q)", where, parseErr.LastKey)
	}

	return fmt.Errorf(
		"failed to parse the development account roster %s at %s; the parser's message is omitted because it can quote the file, which holds the shared password",
		path, where,
	)
}

// joinTOMLKeysは、エラーメッセージ用にキーを並べます。
func joinTOMLKeys(keys []toml.Key) string {
	ss := make([]string, 0, len(keys))
	for _, key := range keys {
		ss = append(ss, key.String())
	}

	return strings.Join(ss, ", ")
}

// joinSeedRolesは、エラーメッセージ用に役割を並べます。
func joinSeedRoles(roles []seedRole) string {
	ss := make([]string, 0, len(roles))
	for _, role := range roles {
		ss = append(ss, string(role))
	}

	return strings.Join(ss, ", ")
}
