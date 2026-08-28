-- +goose Up

-- The tables that hold what a community talks about: categories group boards,
-- a board holds threads, and a thread holds posts. The conventions the columns
-- follow (primary keys, DATETIME timestamps, COLLATE NOCASE, ...) are stated in
-- the initial schema and are not repeated here.
--
-- The five tables arrive in one migration because they form one connected
-- piece: threads carries denormalized columns that point at posts, and
-- post_references links posts to posts.
--
-- [Ja] コミュニティが何を話しているかを保持するテーブル群。カテゴリーが掲示板をまとめ、
-- 掲示板がスレッドを、スレッドが投稿を持つ。列が従う規約 (主キー・DATETIME の時刻・
-- COLLATE NOCASE など) は初期スキーマに記載しており、ここでは繰り返さない。
--
-- 5 つのテーブルを 1 本のマイグレーションで作るのは、1 つの繋がったまとまりを成すため
-- である。threads は posts を指す非正規化列を持ち、post_references は posts どうしを
-- 結ぶ。

-- categories groups the boards a community offers. It is not part of a board's
-- address: a board is reached by its own slug, so moving a board to another
-- category does not change any URL.
--
-- slug is the ASCII identifier /c/{slug} resolves and therefore carries
-- COLLATE NOCASE; name is displayed and may be Unicode, which NOCASE does not
-- fold, so it carries none. position is the order the community intends its
-- categories to appear in, which neither name nor creation order can express.
--
-- [Ja] categories はコミュニティが提供する掲示板をまとめる。掲示板のアドレスの一部では
-- ない。掲示板は自身の slug で辿り着くため、掲示板を別のカテゴリーへ移しても URL は
-- 変わらない。
--
-- slug は /c/{slug} が解決する ASCII の識別子であるため COLLATE NOCASE を持つ。name は
-- 表示用で Unicode を許すが、NOCASE は Unicode を畳み込まないため付けない。position は
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

-- boards is where threads are posted. slug is unique across the instance rather
-- than within a category, because /b/{slug} names a board without naming the
-- category it currently sits in.
--
-- category_id is nullable and ON DELETE SET NULL. A board outside every category
-- is a normal state rather than a gap waiting to be filled (ADR 0011), so
-- deleting a category returns its boards to that state instead of taking them
-- with it or waiting for an operator to move them somewhere first. The index on
-- (category_id, position) is what a category's page follows to list its boards
-- in order, and its leading column is also how ON DELETE SET NULL and the
-- foreign key check reach their rows without scanning the table.
--
-- [Ja] boards はスレッドが立つ場所である。slug がカテゴリー内ではなくインスタンス全体で
-- 一意なのは、/b/{slug} が掲示板を、それが今どのカテゴリーに属するかを言わずに名指しする
-- ためである。
--
-- category_id は nullable かつ ON DELETE SET NULL とする。どのカテゴリーにも属さない掲示板は
-- 埋めるべき欠落ではなく正常な状態であり (ADR 0011)、カテゴリーの削除は配下の掲示板を
-- 巻き込むことも、運営が移し先を決めるのを待つこともせず、それらをその状態へ戻す。
-- (category_id, position) のインデックスはカテゴリーのページがそのカテゴリーの掲示板を順に
-- 並べるときにたどるもので、その先頭カラムは ON DELETE SET NULL と外部キーの検査がテーブルを
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

-- threads is a container for posts and holds no body of its own. It is
-- addressed by id rather than a slug because its title can be edited, and a
-- title-derived address would break the links already shared.
--
-- posts_count, last_post_id and last_posted_at are denormalized from posts. The
-- board's thread list needs all three to render a row, and computing them per
-- row would mean one aggregate per thread on every page view. They are updated
-- in the transaction that writes the post, so keeping them costs no extra write
-- transaction. last_posted_at is NOT NULL because a thread is created together
-- with its first post and so never exists without one; last_post_id is nullable
-- only to survive that post being deleted. Its index lets ON DELETE SET NULL
-- locate the thread without scanning every thread in the instance.
--
-- last_post_id references posts, which this migration creates further down.
-- SQLite resolves the target of a foreign key when a row is written rather than
-- when the table is declared, so the order the statements appear in does not
-- matter.
--
-- board_id is ON DELETE CASCADE: once a board is deleted its contents carry no
-- meaning of their own. user_id is ON DELETE SET NULL because a withdrawn
-- account's row is eventually removed by the purge job, and a cascade would
-- take other people's replies -- whole conversations -- along with the author.
-- The index on user_id keeps that purge from scanning the table.
--
-- [Ja] threads は投稿の入れ物で、自身は本文を持たない。slug ではなく id で指すのは、
-- タイトルが編集されうるためで、タイトルから導いたアドレスでは既に共有されたリンクが
-- 壊れる。
--
-- posts_count・last_post_id・last_posted_at は posts からの非正規化である。掲示板の
-- スレッド一覧は 1 行を描くのにこの 3 つをいずれも必要とし、行ごとに求めるとページを
-- 開くたびにスレッド 1 件につき 1 回の集計が走る。これらは投稿を書き込むトランザクションの
-- 中で更新するため、保持しても書き込みトランザクションは増えない。last_posted_at が
-- NOT NULL なのは、スレッドが最初の投稿と同時に作られ、投稿を持たない状態で存在しない
-- ためである。last_post_id が nullable なのは、その投稿が削除された場合に耐えるためだけ
-- である。
-- この列のインデックスにより、ON DELETE SET NULL はインスタンス内の全スレッドを
-- 走査せずに該当するスレッドを引ける。
--
-- last_post_id が参照する posts は、このマイグレーションの後ろで作られる。SQLite は
-- 外部キーの参照先をテーブルの宣言時ではなく行の書き込み時に解決するため、文の並ぶ順序は
-- 問題にならない。
--
-- board_id は ON DELETE CASCADE とする。掲示板を消すと決めた時点で、その中身は独立した
-- 意味を持たない。user_id を ON DELETE SET NULL とするのは、退会したアカウントの行を
-- いずれパージジョブが物理削除するためで、CASCADE では作者と一緒に他人の返信ごと
-- (会話そのものを) 巻き込むことになる。user_id のインデックスは、そのパージがテーブルを
-- 走査しないようにする。
CREATE TABLE threads (
    id INTEGER PRIMARY KEY,
    board_id INTEGER NOT NULL REFERENCES boards (id) ON DELETE CASCADE,
    user_id INTEGER REFERENCES users (id) ON DELETE SET NULL,
    title TEXT NOT NULL,
    posts_count INTEGER NOT NULL DEFAULT 0,
    last_post_id INTEGER REFERENCES posts (id) ON DELETE SET NULL,
    last_posted_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE INDEX index_threads_on_board_id_and_last_posted_at ON threads (board_id, last_posted_at DESC);

CREATE INDEX index_threads_on_last_post_id ON threads (last_post_id);

CREATE INDEX index_threads_on_user_id ON threads (user_id);

-- posts holds what people write. number is the reply number within the thread
-- and UNIQUE (thread_id, number) is what makes it a permanent address: a >>N in
-- a body, the #p{number} anchor, and a URL shared elsewhere all resolve through
-- the same number. That is only tenable because a thread is capped at 1000
-- posts and its post list is not paginated (ADR 0009); a paginated list would
-- have made the page number, which shifts with the page size, the address
-- instead.
--
-- body stores the text exactly as it was entered, with no markup applied.
-- Rendering (linking >>N and URLs) happens on the way out, so notation can be
-- added later by changing the renderer alone, without rewriting what is stored.
--
-- [Ja] posts は人々が書いたものを保持する。number はスレッド内のレス番号で、これを
-- 永久アドレスにしているのが UNIQUE (thread_id, number) である。本文中の >>N、アンカーの
-- #p{number}、外部で共有された URL が、いずれも同じ番号で解決する。これが成り立つのは
-- スレッドの投稿数に 1000 件の上限があり、投稿一覧をページ分割しないためである
-- (ADR 0009)。ページ分割していれば、ページサイズによってずれるページ番号のほうが
-- アドレスになっていた。
--
-- body は入力されたテキストをそのまま保存し、記法は適用しない。描画 (>>N と URL の
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

-- post_references records that one post refers to another, extracted from the
-- body when the post is saved. Reading it back is how a post learns which later
-- posts replied to it, and the index on referenced_post_id is what that lookup
-- follows.
--
-- The table exists rather than the reverse links being derived from the bodies
-- on screen, because a screen that shows only part of a thread (a single post,
-- or the latest N) would miss every reference made by a post it does not carry.
-- UNIQUE (post_id, referenced_post_id) keeps one relationship to at most one
-- row, so a body that writes >>5 twice has its references deduplicated before
-- they are inserted.
--
-- [Ja] post_references は、ある投稿が別の投稿を参照していることを記録する。値は投稿の
-- 保存時に本文から抽出する。これを読み返すことで、投稿は自分に返信した後続の投稿を知る。
-- その引き当てがたどるのが referenced_post_id のインデックスである。
--
-- 画面に出ている本文から逆リンクを組み立てるのではなくテーブルを持つのは、スレッドの
-- 一部だけを表示する画面 (投稿単体や最新 N 件) では、そこに載っていない投稿からの参照が
-- すべて欠けるためである。UNIQUE (post_id, referenced_post_id) は 1 つの関係を高々 1 行に
-- 保つため、>>5 を 2 度書いた本文は、参照を重複除去してから INSERT する。
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
