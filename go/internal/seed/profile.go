package seed

// Profileは、実行が生成するコミュニティの状態です。コミュニティは1つではなく
// 2つの状態で眺めます。画面を設計したときの成熟した状態と、どのインスタンスも必ず通る
// 立ち上げ直後の状態 (掲示板1つ・スレッド数本・各数レス) です (ADR 0010)。どちらか
// 一方の大きさでしか成立しない画面は欠陥であり、それを見つけるには両方を生成する必要が
// あります。
//
// フィールドを非公開にしているのは、実行が作れる状態をここに書かれたものに限るため
// です。呼び出し側は名前で1つを指定することになり、誰も書いていない状態をパッケージの
// 外から組み立てることはできません。
type Profile struct {
	name          string
	communityName string
	categories    []seedCategory
	boards        []seedBoard
	plan          contentPlan

	// scriptsは投稿ごとに書き下したスレッドです。生成器ではなくプロファイルが
	// 持つのは、そのどれをコミュニティが持つのかが、続いてきた長さによって変わるから
	// です。立ち上げ直後のコミュニティは、それらが見せるやり取りをまだ持ちません。
	// どのプロファイルにも入る台本も各プロファイルに書き並べるのは、プロファイルが何を
	// 書き下すのかを、そのプロファイルだけで読めるようにするためです。
	scripts []scriptedThread
}

// matureProfileは、画面を作りながら見るコミュニティです。カテゴリーの下と外に
// またがる複数の掲示板、スレッド一覧の1ページに収まらない数のスレッドを持つ掲示板、
// そして1つずつ開いて眺めるために書き下したスレッドを備えます。
var matureProfile = Profile{
	name:          "mature",
	communityName: matureCommunityName,
	categories:    matureCategories,
	boards:        matureBoards,
	plan:          matureContentPlan,
	scripts:       []scriptedThread{referenceScript, withdrawnScript, englishScript, otherLanguageScript},
}

// coldStartProfileは立ち上げ直後のコミュニティです。どのインスタンスも必ず通る
// 状態であり、そのため画面はこれに照らして確かめる必要があります (ADR 0010)。
// カテゴリーも、投稿数の上限に近いスレッドも、退会した作者が残した投稿もありません。
// 開いたばかりのコミュニティには、そのいずれもまだ存在しないためです。
//
// 唯一持つ書き下しのスレッドが英語のスレッドです。ここで省くのはコミュニティが蓄積する
// ものであり、複数の言語が並ぶ掲示板はそれに当たりません。ラウンジは開いた日から日本語と
// 英語を受け付けています。
var coldStartProfile = Profile{
	name:          "cold-start",
	communityName: coldStartCommunityName,
	boards:        coldStartBoards,
	plan:          coldStartContentPlan,
	scripts:       []scriptedThread{englishScript},
}

// profilesは、コマンドラインが名指しできるプロファイルを、既定のものを先頭にして
// 並べたものです。
var profiles = []Profile{matureProfile, coldStartProfile}

// DefaultProfileは、コマンドラインが何も指定しないときに実行が生成する
// プロファイルを返します。それは上の一覧の先頭に書かれたものです。成熟したコミュニティが
// そこにあるのは、画面を眺められる形をすべて備えているのがその状態だからです。立ち上げ
// 直後の状態は、それを確かめたいときに指定します。既定を一覧から読むことで、usageの行と
// 既定が同じ状態を名指すようになります。
func DefaultProfile() Profile {
	return profiles[0]
}

// FindProfileはnameの名前で書かれたプロファイルを返し、それが存在するかどうかを
// 報告します。何も書かれていない名前に対して既定のものを生成せずここで答えるのは、実行が
// データベースを空にするためです。打ち間違えたプロファイルは、何かが削除される前に失敗
// する必要があり、指定されたのとは別の状態を作ってはなりません。
func FindProfile(name string) (Profile, bool) {
	for _, profile := range profiles {
		if profile.name == name {
			return profile, true
		}
	}

	return Profile{}, false
}

// ProfileNamesはFindProfileが応じる名前を挙げます。groobb devcredsのusageが
// SignInRolesから組み立てられるのと同じく、groobb seedのusageをここから組み立てる
// ことで、その1行が、受け付けない名前を指定して探し当てるのではなく、本サブコマンドが
// 生成する状態そのものを挙げられるようになります。
func ProfileNames() []string {
	names := make([]string, 0, len(profiles))
	for _, profile := range profiles {
		names = append(names, profile.name)
	}

	return names
}

// Nameは、コマンドラインがこのプロファイルを名指しするときの名前を返します。
// プロファイルについてパッケージの外から読めるのはこれだけです。どのコミュニティを生成
// するのかはシードの領分であり、どれを求めたのかは呼び出し側の領分であるためです。
func (p Profile) Name() string {
	return p.name
}
