package seed

// Profile is the state of a community a run generates. A community is looked at
// in two states rather than one: the mature state the screens were designed
// against, and the state every instance opens in, with a single board, a few
// threads and a few posts in each (ADR 0010). A screen that only holds together
// at one of the two sizes is a defect, and finding it takes generating both.
//
// The fields are unexported so that the states a run can produce are the ones
// written here. A caller names one, and a state nobody wrote cannot be
// assembled from outside the package.
//
// [Ja] Profile は、実行が生成するコミュニティの状態です。コミュニティは 1 つではなく
// 2 つの状態で眺めます。画面を設計したときの成熟した状態と、どのインスタンスも必ず通る
// 立ち上げ直後の状態 (掲示板 1 つ・スレッド数本・各数レス) です (ADR 0010)。どちらか
// 一方の大きさでしか成立しない画面は欠陥であり、それを見つけるには両方を生成する必要が
// あります。
//
// フィールドを非公開にしているのは、実行が作れる状態をここに書かれたものに限るため
// です。呼び出し側は名前で 1 つを指定することになり、誰も書いていない状態をパッケージの
// 外から組み立てることはできません。
type Profile struct {
	name          string
	communityName string
	categories    []seedCategory
	boards        []seedBoard
	plan          contentPlan

	// scripts are the threads written out post by post. The profile holds them
	// rather than the generator because they are content a community has
	// accumulated: on its first days it has neither the exchanges they show nor
	// the busy board they are posted in.
	//
	// [Ja] scripts は投稿ごとに書き下したスレッドです。生成器ではなくプロファイルが
	// 持つのは、それらがコミュニティの蓄積した中身だからです。立ち上げ直後の
	// コミュニティは、それらが見せるやり取りも、それらが立つ賑わった掲示板も持ちません。
	scripts []scriptedThread
}

// matureProfile is the community the screens are worked on against: several
// boards under and outside categories, a board holding more threads than one
// page of a thread list, and the threads written out to be opened one by one.
//
// [Ja] matureProfile は、画面を作りながら見るコミュニティです。カテゴリーの下と外に
// またがる複数の掲示板、スレッド一覧の 1 ページに収まらない数のスレッドを持つ掲示板、
// そして 1 つずつ開いて眺めるために書き下したスレッドを備えます。
var matureProfile = Profile{
	name:          "mature",
	communityName: matureCommunityName,
	categories:    matureCategories,
	boards:        matureBoards,
	plan:          matureContentPlan,
	scripts:       []scriptedThread{referenceScript, withdrawnScript},
}

// coldStartProfile is the community on its first days, which every instance
// passes through and which is therefore what a screen has to be checked against
// (ADR 0010). It has no categories, no thread near the post limit and no post
// left behind by an author who has withdrawn, because none of those exist yet in
// a community that has just opened.
//
// [Ja] coldStartProfile は立ち上げ直後のコミュニティです。どのインスタンスも必ず通る
// 状態であり、そのため画面はこれに照らして確かめる必要があります (ADR 0010)。
// カテゴリーも、投稿数の上限に近いスレッドも、退会した作者が残した投稿もありません。
// 開いたばかりのコミュニティには、そのいずれもまだ存在しないためです。
var coldStartProfile = Profile{
	name:          "cold-start",
	communityName: coldStartCommunityName,
	boards:        coldStartBoards,
	plan:          coldStartContentPlan,
}

// profiles lists the profiles a command line can name, the default first.
//
// [Ja] profiles は、コマンドラインが名指しできるプロファイルを、既定のものを先頭にして
// 並べたものです。
var profiles = []Profile{matureProfile, coldStartProfile}

// DefaultProfile returns the profile a run generates when a command line names
// none, which is the first one written above. The mature community is the one
// there because it is the state that holds every shape a screen can be looked
// at in; the first-day state is asked for when that is what is being checked.
// Reading the default off the list is what keeps the usage line and the default
// naming the same state.
//
// [Ja] DefaultProfile は、コマンドラインが何も指定しないときに実行が生成する
// プロファイルを返します。それは上の一覧の先頭に書かれたものです。成熟したコミュニティが
// そこにあるのは、画面を眺められる形をすべて備えているのがその状態だからです。立ち上げ
// 直後の状態は、それを確かめたいときに指定します。既定を一覧から読むことで、usage の行と
// 既定が同じ状態を名指すようになります。
func DefaultProfile() Profile {
	return profiles[0]
}

// FindProfile returns the profile written under name, and reports whether one
// is. A name nothing is written under is answered here rather than by generating
// the default: a run empties the database, so a mistyped profile has to fail
// before anything is deleted, not produce a state other than the one asked for.
//
// [Ja] FindProfile は name の名前で書かれたプロファイルを返し、それが存在するかどうかを
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

// ProfileNames lists the names FindProfile answers to. The usage of groobb seed
// is built from this, the way the usage of groobb devcreds is built from
// SignInRoles, so that the line names the states the subcommand generates
// instead of leaving them to be found by naming one it does not.
//
// [Ja] ProfileNames は FindProfile が応じる名前を挙げます。groobb devcreds の usage が
// SignInRoles から組み立てられるのと同じく、groobb seed の usage をここから組み立てる
// ことで、その 1 行が、受け付けない名前を指定して探し当てるのではなく、本サブコマンドが
// 生成する状態そのものを挙げられるようになります。
func ProfileNames() []string {
	names := make([]string, 0, len(profiles))
	for _, profile := range profiles {
		names = append(names, profile.name)
	}

	return names
}

// Name returns the name a command line names this profile by. It is the only
// thing about a profile that is readable from outside the package: which
// community a run generates is the seed's business, while which one was asked
// for is the caller's.
//
// [Ja] Name は、コマンドラインがこのプロファイルを名指しするときの名前を返します。
// プロファイルについてパッケージの外から読めるのはこれだけです。どのコミュニティを生成
// するのかはシードの領分であり、どれを求めたのかは呼び出し側の領分であるためです。
func (p Profile) Name() string {
	return p.name
}
