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

// rosterPath is the file a run reads its accounts from, and rosterExamplePath
// is the example committed in its place. Both are relative to the directory a
// run starts in, which is the Go module root (see the seed target in
// go/Makefile).
//
// The roster holds personal email addresses, so it is kept out of version
// control and the example is committed instead. The example says which accounts
// a development environment has and what each one is there to show, without
// naming anybody.
//
// [Ja] rosterPath は実行がアカウントを読み込むファイル、rosterExamplePath はその代わりに
// コミットしている見本です。どちらも実行を開始したディレクトリからの相対パスであり、
// それは Go モジュールのルートになります (go/Makefile の seed ターゲットを参照)。
//
// 名簿は個人のメールアドレスを持つためバージョン管理には入れず、代わりに見本を
// コミットしています。見本は誰かを名指しすることなく、開発環境にどんなアカウントがいて、
// それぞれが何を確認するためにいるのかを説明します。
const (
	rosterPath        = "seed-users.toml"
	rosterExamplePath = "seed-users.example.toml"
)

// rosterFile is the roster as it is written in the file.
//
// [Ja] rosterFile は、ファイルに書かれたままの名簿です。
type rosterFile struct {
	// Password is shared by every account. The seed refuses to run outside a
	// development environment and the dev site itself sits behind basic auth, so
	// a password per account would buy nothing and cost one password manager
	// entry per account.
	//
	// [Ja] Password は全アカウントで共通です。シードは開発環境以外での実行を拒否し、
	// dev サイト自体も Basic 認証の内側にあるため、アカウントごとに別のパスワードを
	// 持たせても得るものが無く、アカウントの数だけパスワード管理の項目が増えます。
	Password string           `toml:"password"`
	Users    []rosterUserFile `toml:"users"`
}

// rosterUserFile is one [[users]] entry as it is written in the file.
//
// [Ja] rosterUserFile は、ファイルに書かれたままの [[users]] 1 件です。
type rosterUserFile struct {
	Role   string `toml:"role"`
	Atname string `toml:"atname"`
	Email  string `toml:"email"`
	Note   string `toml:"note"`
}

// rosterUser is one account the roster names.
//
// [Ja] rosterUser は、名簿が挙げるアカウント 1 件です。
type rosterUser struct {
	role   seedRole
	atname string
	email  string
	// note says what the account is there to look at. A run reports it beside
	// the address that signs the account in, so that the developer picks an
	// account by what it shows rather than by remembering which atname wrote
	// what.
	//
	// [Ja] note は、そのアカウントが何を見るためにいるのかを述べます。実行はこれを、
	// そのアカウントをサインインさせるアドレスと並べて報告します。開発者が、どの atname が
	// 何を書いたかを覚えているかどうかではなく、何を見せるアカウントなのかで選べるように
	// するためです。
	note string
}

// userRoster is the accounts a run creates. Who exists is configuration rather
// than code: the addresses are personal, and an account is added to look at a
// screen from a viewpoint the existing ones cannot take. Which account does
// what stays in the code, which reaches for them by role.
//
// [Ja] userRoster は実行が作成するアカウントです。誰がいるのかはコードではなく設定と
// します。アドレスが個人のものであることと、アカウントが足されるのは既存のアカウントでは
// 取れない視点から画面を見るためであることによります。どのアカウントが何をするのかは
// コードに残り、コードはアカウントを役割で引きます。
type userRoster struct {
	// path is the file the roster was read from. A run reports it beside the
	// database it is about to empty, so that both of the things it was pointed
	// at can be read off one line.
	//
	// [Ja] path は名簿を読み込んだファイルです。実行はこれを、これから空にする
	// データベースと並べて報告します。実行が何を向いているのかを 1 行で読み取れるように
	// するためです。
	path string
	// passwordDigest is the shared password as the accounts store it. The
	// plaintext is not carried past reading: what a run writes is the digest,
	// and hashing once while the roster is read is what lets the plaintext be
	// dropped.
	//
	// [Ja] passwordDigest は、アカウントが保存する形にした共通パスワードです。平文は
	// 読み込みの先へは持ち越しません。実行が書き込むのはダイジェストであり、名簿の
	// 読み込み時に一度ハッシュ化していることが、平文を落とせる理由になります。
	passwordDigest string
	users          []rosterUser
}

// loadUserRoster reads the roster from path.
//
// A missing file is an error rather than a fall back to the example. The
// example carries placeholder addresses, and signing in as an account nobody
// reads mail for is not a state to arrive at by accident.
//
// [Ja] loadUserRoster は path から名簿を読みます。
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

// loadRosterFile reads the roster at path as it is written, without checking
// what it holds.
//
// [Ja] loadRosterFile は path の名簿を、書かれたままの形で読みます。中身が何であるかの
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

	// A key the decoder did not use is a typo: the value written next to it
	// never reaches the account, and the run goes on to create that account
	// without whatever the key was meant to give it.
	//
	// [Ja] デコーダが使わなかったキーは書き間違いです。その隣に書かれた値はアカウントへ
	// 届かず、実行はそのキーが与えるはずだったものを欠いたまま、そのアカウントを作りに
	// 行きます。
	if keys := meta.Undecoded(); len(keys) > 0 {
		return rosterFile{}, fmt.Errorf("the development account roster %s has keys that do not exist: %s", path, joinTOMLKeys(keys))
	}

	return file, nil
}

// toUserRoster checks the roster over and returns what the generators work
// from.
//
// [Ja] toUserRoster は名簿を検査し、生成器が使う形にして返します。
func (f rosterFile) toUserRoster(path string) (*userRoster, error) {
	users, err := f.validate()
	if err != nil {
		return nil, err
	}

	// Hash the shared password while the roster is still being read. Besides
	// rejecting an input bcrypt cannot handle before the database is touched,
	// this lets every account use the same prepared digest instead of a hashing
	// failure surfacing after some of the accounts have already been written.
	//
	// [Ja] 名簿を読み込んでいる間に共通パスワードをハッシュ化します。bcrypt が処理
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

// validate checks the roster over and returns the accounts it holds.
//
// It stops short of the password digest, which is what separates it from
// toUserRoster. Reading the roster to hand one account's credentials to the
// browser verification wants the password as it is written, and hashing it
// there would buy nothing for the wait it costs.
//
// [Ja] validate は名簿を検査し、そこに書かれているアカウントを返します。
//
// パスワードダイジェストの手前で止まる点が toUserRoster との違いです。1 件分の資格情報を
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

		// The role is what a generator names an account by, the atname is the
		// handle a post is attributed to, and the email is what the browser
		// verification signs in with. A value written twice makes one of the two
		// accounts unreachable through whichever of the three they share.
		//
		// Both columns collate NOCASE, so two values differing only in letter
		// case are the same value to the UNIQUE constraint the second account
		// would be written against. Comparing them the same way here keeps that
		// collision from surfacing as a failed insert after the database has
		// been emptied.
		//
		// [Ja] 役割は生成器がアカウントを名指しする名前、atname は投稿が誰のものかを示す
		// ハンドル、メールアドレスはブラウザ確認がサインインに使う名前です。同じ値を
		// 2 度書くと、共有した名前ではどちらか一方のアカウントへ辿り着けなくなります。
		//
		// どちらの列も NOCASE 照合のため、大文字小文字だけが違う 2 つの値は、2 件目の
		// アカウントが書き込まれる UNIQUE 制約にとって同じ値です。ここでも同じ方法で
		// 比較することで、この衝突がデータベースを空にした後の INSERT 失敗として表面化
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

	// A role the generators name but the roster does not hold would not surface
	// until the generator that needs it runs, which is after the run has emptied
	// the database.
	//
	// [Ja] 生成器が名指ししているのに名簿に無い役割は、それを必要とする生成器が走るまで
	// 表面化せず、それは実行がデータベースを空にした後になります。
	for _, role := range allSeedRoles {
		if !roles[role] {
			return nil, fmt.Errorf("there is no [[users]] entry with the role %s; the generators name this role, so one is required", role)
		}
	}

	return users, nil
}

// toRosterUser checks one entry over and returns it in the form the generators
// work from.
//
// [Ja] toRosterUser は 1 件分を検査し、生成器が使う形にして返します。
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

	// The atname is checked against the rule the application applies to every
	// account, rather than against a copy of it here, so that an atname the
	// roster accepts is one an account can hold. Checking it while the roster is
	// read keeps an invalid account from being found only after the database has
	// been emptied.
	//
	// [Ja] atname は、ここに置いた写しではなくアプリケーションがすべてのアカウントに
	// 課している規則で検査します。名簿が受理する atname が、アカウントが実際に持てる
	// atname であるようにするためです。名簿の読み込み時に検査することで、データベースを
	// 空にした後で初めて不正なアカウントが見つかることを防ぎます。
	if !validator.IsValidAtname(e.Atname) {
		return rosterUser{}, fmt.Errorf(
			"the atname %q may hold only ASCII letters, digits and underscores, and at most %d of them",
			e.Atname, validator.AtnameMaxLength,
		)
	}

	// An email the roster accepts must also be the address that signs the
	// account in. The roster stores what is written rather than what parsing
	// makes of it, while surrounding whitespace and a display name parse away:
	// an account written either way would hold an address the sign-in form,
	// whose field drops the same whitespace, cannot submit. Requiring the
	// written form to be the address itself keeps a run from finishing with an
	// account nobody can reach.
	//
	// [Ja] 名簿が受理するメールアドレスは、そのアカウントをサインインさせるアドレスその
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

	// The note is the one required string with no format of its own to check it
	// against: the atname's rule rejects whitespace and the email has to be
	// written as the address itself, while a note is whatever it says. Trimming
	// it keeps a stray space in the file from becoming a report line that reads
	// as indented.
	//
	// [Ja] note は、照らし合わせる形式を自身では持たない唯一の必須文字列です。atname は
	// 規則が空白を弾き、email は書かれた文字列がアドレスそのものであることを求められますが、
	// 覚え書きは書かれたものが何であれ覚え書きになります。トリムすることで、ファイルに
	// 紛れ込んだ空白が、字下げされて見える報告行になることを防ぎます。
	return rosterUser{
		role:   role,
		atname: e.Atname,
		email:  e.Email,
		note:   strings.TrimSpace(e.Note),
	}, nil
}

// rosterFileError converts a failure to read or decode the roster into an error
// that names where the problem is without repeating what the file says there.
//
// A syntax error's message quotes the token the parser stumbled on (`expected
// value but found "hunter" instead` for an unquoted password), and this error is
// logged, so the message is dropped and only the position and the last key
// parsed are kept. The roster holds the password every account signs in with.
// Everything else is passed through: the remaining decoding errors report types
// rather than values.
//
// [Ja] rosterFileError は、名簿の読み込み・デコードの失敗を、問題の位置は示しつつ
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

// joinTOMLKeys lists keys for an error message.
//
// [Ja] joinTOMLKeys は、エラーメッセージ用にキーを並べます。
func joinTOMLKeys(keys []toml.Key) string {
	ss := make([]string, 0, len(keys))
	for _, key := range keys {
		ss = append(ss, key.String())
	}

	return strings.Join(ss, ", ")
}

// joinSeedRoles lists roles for an error message.
//
// [Ja] joinSeedRoles は、エラーメッセージ用に役割を並べます。
func joinSeedRoles(roles []seedRole) string {
	ss := make([]string, 0, len(roles))
	for _, role := range roles {
		ss = append(ss, string(role))
	}

	return strings.Join(ss, ", ")
}
