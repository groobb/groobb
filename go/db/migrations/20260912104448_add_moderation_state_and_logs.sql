-- +goose Up

-- The four states an administrator can put a thread, a post or an account into,
-- and the record of who put it there. They arrive in one migration because the
-- record refers to all of them: a moderation_logs row names the thread, post or
-- account the operation was aimed at, so the columns and the table it points at
-- are one piece.
--
-- Every state is a timestamp rather than a boolean, so that a state a row
-- carries can be matched against the logged operation that produced it. A
-- boolean would say that a thread is locked without saying since when, leaving
-- the log as the only place the moment exists and nothing to line the two up by.
--
-- The column conventions these follow (DATETIME timestamps, the fixed-width
-- ISO8601 default, primary keys) are stated in the initial schema and are not
-- repeated here.
--
-- [Ja] 管理者がスレッド・投稿・アカウントを置ける4つの状態と、誰がそこへ置いたかの記録。
-- 1本のマイグレーションにまとめるのは、記録がそのすべてを参照するためである。
-- moderation_logsの行は操作の対象となったスレッド・投稿・アカウントを名指すため、
-- 列とその指す先のテーブルは1つのまとまりを成す。
--
-- どの状態も真偽値ではなく時刻で持つ。行が持つ状態を、それを生んだ操作の記録と突き合わせ
-- られるようにするためである。真偽値はスレッドがロックされていることを述べるが、いつから
-- かは述べない。時刻が記録の側にしか存在しなくなり、両者を並べる手がかりが無くなる。
--
-- これらが従う列の規約 (DATETIMEの時刻・桁数固定のISO8601の既定値・主キー) は初期スキーマに
-- 記載しており、ここでは繰り返さない。

-- locked_at is the moderator's lock and sits apart from the post cap, which is
-- derived from posts_count and is not stored. The two hold alongside each other,
-- so unlocking clears this column alone and a thread that also filled up stays
-- closed for that reason.
--
-- [Ja] locked_atは管理者によるロックで、posts_countから導かれ保存されない上限到達とは
-- 別に置く。2つは並び立つため、解除が外すのはこの列だけであり、併せて満杯になっている
-- スレッドはその理由で閉じたままになる。
ALTER TABLE threads ADD COLUMN locked_at DATETIME;

-- unpublished_at is named for what it does rather than deleted_at, because the
-- two are different fates. deleted_at marks a row on its way to a purge, while
-- an unpublished thread or post keeps its title and body indefinitely: the mark
-- hides it, and taking the mark off brings it back.
--
-- [Ja] unpublished_atという名前はそれが行うことに由来するもので、deleted_atとは別の行き先
-- を表す。deleted_atはパージへ向かう行の印だが、非公開のスレッドや投稿はタイトルと本文を
-- 保ち続ける。印は対象を隠すものであり、印を外せば戻る。
ALTER TABLE threads ADD COLUMN unpublished_at DATETIME;

ALTER TABLE posts ADD COLUMN unpublished_at DATETIME;

-- suspended_at sits apart from users.deleted_at because suspension and
-- withdrawal leave an account in different places. A suspended account keeps its
-- identity -- its email and atname are untouched -- and only its activity stops,
-- while a withdrawn one is anonymized and heads for the purge. One column
-- holding both would make lifting a suspension indistinguishable from undoing a
-- withdrawal.
--
-- [Ja] suspended_atをusers.deleted_atとは別に置くのは、停止と退会がアカウントを別の場所に
-- 置くためである。停止されたアカウントは身元を保ち (emailもatnameもそのままである)、
-- 止まるのは活動だけだが、退会したアカウントは匿名化されパージへ向かう。1つの列が両方を
-- 持てば、停止の解除と退会の取り消しを区別できなくなる。
ALTER TABLE users ADD COLUMN suspended_at DATETIME;

-- moderation_logs records the operations that produced those states, so that
-- what an administrator did is answerable after the fact from something other
-- than the state itself. A state says how a row stands now; the log says who
-- brought it there, when, and why.
--
-- The target is held in a column per kind of target rather than as a
-- target_type / target_id pair. The kinds are three and closed, so the foreign
-- keys keep the reference honest and the listing joins to the rows it displays
-- without a condition on the type first.
--
-- user_id is the administrator who acted and is nullable for two reasons that
-- are deliberately not told apart: an operator working outside any account has
-- no row to point at, and a withdrawn account's row is eventually removed by the
-- purge job. The atname of a withdrawn account is already anonymized at the
-- moment of withdrawal, so nothing readable is lost when the reference goes.
--
-- Each of the four foreign keys carries an index, which is what ON DELETE SET
-- NULL follows to reach the rows referring to a deleted thread, post or account
-- instead of scanning the whole log. The listing itself reads by primary key
-- descending and needs no index of its own.
--
-- action is not enumerated in a CHECK, for the reason threads.language is not:
-- SQLite cannot alter one, so every operation added would take a migration that
-- rebuilds the table and copies its rows. The set lives in
-- model.ModerationActions and the repository applies it before the insert.
--
-- reason defaults to the empty string because giving a reason is optional, and
-- an empty string says the administrator wrote none just as well as NULL would
-- while sparing every reader the distinction.
--
-- [Ja] moderation_logsは、それらの状態を生んだ操作を記録する。管理者が何を行ったかを、
-- 状態そのもの以外から後から答えられるようにするためである。状態は行が今どうなっているかを
-- 述べ、記録は誰がいつ何のためにそこへ置いたかを述べる。
--
-- 対象はtarget_type / target_idの組ではなく、対象の種類ごとの列で持つ。種類は3つで閉じて
-- いるため、外部キーが参照の正しさを保ち、一覧は種類による場合分けを挟まずに表示する行へ
-- 結合できる。
--
-- user_idは操作した管理者で、意図的に区別しない2つの理由からnullableである。どのアカウント
-- でもなく運用者として行った操作は指す行を持たず、退会したアカウントの行はいずれパージ
-- ジョブが物理削除する。退会したアカウントのatnameは退会の時点で既に匿名化されているため、
-- 参照が外れても読み取れるものは失われない。
--
-- 4つの外部キーはそれぞれ索引を持つ。ON DELETE SET NULLが、削除されたスレッド・投稿・
-- アカウントを参照する行へ、記録全体を走査せずに届くためにたどるものである。一覧自体は
-- 主キーの降順で読むため、専用の索引は要らない。
--
-- actionをCHECKで列挙しないのはthreads.languageと同じ理由による。SQLiteはCHECKを変更できず、
-- 操作を1つ足すたびにテーブルを作り直して行を移すマイグレーションが必要になる。値域は
-- model.ModerationActionsに持ち、リポジトリが挿入の前にそれを適用する。
--
-- reasonの既定値が空文字列なのは、理由を述べることが任意であるためである。空文字列は
-- NULLと同じだけ「管理者が何も書かなかった」ことを述べ、しかも読み手に区別を強いない。
CREATE TABLE moderation_logs (
    id INTEGER PRIMARY KEY,
    user_id INTEGER REFERENCES users (id) ON DELETE SET NULL,
    action TEXT NOT NULL,
    thread_id INTEGER REFERENCES threads (id) ON DELETE SET NULL,
    post_id INTEGER REFERENCES posts (id) ON DELETE SET NULL,
    target_user_id INTEGER REFERENCES users (id) ON DELETE SET NULL,
    reason TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE INDEX index_moderation_logs_on_user_id ON moderation_logs (user_id);

CREATE INDEX index_moderation_logs_on_thread_id ON moderation_logs (thread_id);

CREATE INDEX index_moderation_logs_on_post_id ON moderation_logs (post_id);

CREATE INDEX index_moderation_logs_on_target_user_id ON moderation_logs (target_user_id);

-- +goose Down

DROP TABLE moderation_logs;

ALTER TABLE users DROP COLUMN suspended_at;

ALTER TABLE posts DROP COLUMN unpublished_at;

ALTER TABLE threads DROP COLUMN unpublished_at;

ALTER TABLE threads DROP COLUMN locked_at;
