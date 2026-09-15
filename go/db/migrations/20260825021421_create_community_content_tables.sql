-- +goose Up

-- コミュニティが何を話しているかを保持するテーブル群。カテゴリーが掲示板をまとめ、
-- 掲示板がスレッドを、スレッドが投稿を持つ。列が従う規約 (主キー・DATETIMEの時刻・
-- COLLATE NOCASEなど) は初期スキーマに記載しており、ここでは繰り返さない。
--
-- 5つのテーブルを1本のマイグレーションで作るのは、1つの繋がったまとまりを成すため
-- である。threadsはpostsを指す非正規化列を持ち、post_referencesはpostsどうしを
-- 結ぶ。

-- categoriesはコミュニティが提供する掲示板をまとめる。掲示板のアドレスの一部では
-- ない。掲示板は自身のslugで辿り着くため、掲示板を別のカテゴリーへ移してもURLは
-- 変わらない。
--
-- slugは /c/{slug} が解決するASCIIの識別子であるためCOLLATE NOCASEを持つ。nameは
-- 表示用でUnicodeを許すが、NOCASEはUnicodeを畳み込まないため付けない。positionは
-- コミュニティがカテゴリーを並べたい順序で、名前順でも作成順でもその意図は表せない。
CREATE TABLE categories (
    id INTEGER PRIMARY KEY,
    slug TEXT NOT NULL COLLATE NOCASE,
    name TEXT NOT NULL,
    position INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    UNIQUE (slug)
);

-- boardsはスレッドが立つ場所である。slugがカテゴリー内ではなくインスタンス全体で
-- 一意なのは、/b/{slug} が掲示板を、それが今どのカテゴリーに属するかを言わずに名指しする
-- ためである。
--
-- category_idはnullableかつON DELETE SET NULLとする。どのカテゴリーにも属さない掲示板は
-- 埋めるべき欠落ではなく正常な状態であり (ADR 0011)、カテゴリーの削除は配下の掲示板を
-- 巻き込むことも、運営が移し先を決めるのを待つこともせず、それらをその状態へ戻す。
-- (category_id, position) のインデックスはカテゴリーのページがそのカテゴリーの掲示板を順に
-- 並べるときにたどるもので、その先頭カラムはON DELETE SET NULLと外部キーの検査がテーブルを
-- 走査せずに対象の行へ届く手立てでもある。
CREATE TABLE boards (
    id INTEGER PRIMARY KEY,
    category_id INTEGER REFERENCES categories (id) ON DELETE SET NULL,
    slug TEXT NOT NULL COLLATE NOCASE,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    position INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    UNIQUE (slug)
);

CREATE INDEX index_boards_on_category_id_and_position ON boards (category_id, position);

-- threadsは投稿の入れ物で、自身は本文を持たない。slugではなくidで指すのは、
-- タイトルが編集されうるためで、タイトルから導いたアドレスでは既に共有されたリンクが
-- 壊れる。
--
-- posts_count・last_post_id・last_posted_atはpostsからの非正規化である。掲示板の
-- スレッド一覧は1行を描くのにこの3つをいずれも必要とし、行ごとに求めるとページを
-- 開くたびにスレッド1件につき1回の集計が走る。これらは投稿を書き込むトランザクションの
-- 中で更新するため、保持しても書き込みトランザクションは増えない。last_posted_atが
-- NOT NULLなのは、スレッドが最初の投稿と同時に作られ、投稿を持たない状態で存在しない
-- ためである。last_post_idがnullableなのは、その投稿が削除された場合に耐えるためだけ
-- である。
-- この列のインデックスにより、ON DELETE SET NULLはインスタンス内の全スレッドを
-- 走査せずに該当するスレッドを引ける。
--
-- last_post_idが参照するpostsは、このマイグレーションの後ろで作られる。SQLiteは
-- 外部キーの参照先をテーブルの宣言時ではなく行の書き込み時に解決するため、文の並ぶ順序は
-- 問題にならない。
--
-- languageはスレッドが書かれている言語で、取りうる値をCHECKで列挙しない。SQLiteは
-- CHECKを変更できず、言語を1つ足すたびにテーブルを作り直して行を移すマイグレーションが
-- 必要になるためである。値域はアプリケーション側 (model.ThreadLanguages) に置き、
-- リポジトリが挿入の前にそれを適用する。既定値も持たせない。コミュニティがどの言語で書くかは
-- そのコミュニティのものでスキーマのものではなく、ここに既定値を置けば、セルフホストされる
-- どのインスタンスもGroobb自身のコミュニティがたまたま使う言語から始まることになる。
--
-- board_idはON DELETE CASCADEとする。掲示板を消すと決めた時点で、その中身は独立した
-- 意味を持たない。user_idをON DELETE SET NULLとするのは、退会したアカウントの行を
-- いずれパージジョブが物理削除するためで、CASCADEでは作者と一緒に他人の返信ごと
-- (会話そのものを) 巻き込むことになる。user_idのインデックスは、そのパージがテーブルを
-- 走査しないようにする。
CREATE TABLE threads (
    id INTEGER PRIMARY KEY,
    board_id INTEGER NOT NULL REFERENCES boards (id) ON DELETE CASCADE,
    user_id INTEGER REFERENCES users (id) ON DELETE SET NULL,
    title TEXT NOT NULL,
    language TEXT NOT NULL,
    posts_count INTEGER NOT NULL DEFAULT 0,
    last_post_id INTEGER REFERENCES posts (id) ON DELETE SET NULL,
    last_posted_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE INDEX index_threads_on_board_id_and_last_posted_at ON threads (board_id, last_posted_at DESC);

CREATE INDEX index_threads_on_last_post_id ON threads (last_post_id);

CREATE INDEX index_threads_on_user_id ON threads (user_id);

-- postsは人々が書いたものを保持する。numberはスレッド内のレス番号で、これを
-- 永久アドレスにしているのがUNIQUE (thread_id, number) である。本文中の >>N、アンカーの
-- #p{number}、外部で共有されたURLが、いずれも同じ番号で解決する。これが成り立つのは
-- スレッドの投稿数に1000件の上限があり、投稿一覧をページ分割しないためである
-- (ADR 0009)。ページ分割していれば、ページサイズによってずれるページ番号のほうが
-- アドレスになっていた。
--
-- bodyは入力されたテキストをそのまま保存し、記法は適用しない。描画 (>>NとURLの
-- リンク化) は取り出す側で行うため、記法は保存されたものを書き換えずに描画側の変更だけで
-- 後から足せる。
CREATE TABLE posts (
    id INTEGER PRIMARY KEY,
    thread_id INTEGER NOT NULL REFERENCES threads (id) ON DELETE CASCADE,
    user_id INTEGER REFERENCES users (id) ON DELETE SET NULL,
    number INTEGER NOT NULL,
    body TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    UNIQUE (thread_id, number)
);

CREATE INDEX index_posts_on_user_id ON posts (user_id);

-- post_referencesは、ある投稿が別の投稿を参照していることを記録する。値は投稿の
-- 保存時に本文から抽出する。これを読み返すことで、投稿は自分に返信した後続の投稿を知る。
-- その引き当てがたどるのがreferenced_post_idのインデックスである。
--
-- 画面に出ている本文から逆リンクを組み立てるのではなくテーブルを持つのは、スレッドの
-- 一部だけを表示する画面 (投稿単体や最新N件) では、そこに載っていない投稿からの参照が
-- すべて欠けるためである。UNIQUE (post_id, referenced_post_id) は1つの関係を高々1行に
-- 保つため、>>5を2度書いた本文は、参照を重複除去してからINSERTする。
CREATE TABLE post_references (
    id INTEGER PRIMARY KEY,
    post_id INTEGER NOT NULL REFERENCES posts (id) ON DELETE CASCADE,
    referenced_post_id INTEGER NOT NULL REFERENCES posts (id) ON DELETE CASCADE,
    created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    UNIQUE (post_id, referenced_post_id)
);

CREATE INDEX index_post_references_on_referenced_post_id ON post_references (referenced_post_id);

-- +goose Down

DROP TABLE post_references;

DROP TABLE posts;

DROP TABLE threads;

DROP TABLE boards;

DROP TABLE categories;
