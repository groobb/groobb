-- +goose Up

-- 管理者がスレッド・投稿・アカウントを置ける4つの状態と、誰がそこへ置いたかの記録。
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

-- locked_atは管理者によるロックで、posts_countから導かれ保存されない上限到達とは
-- 別に置く。2つは並び立つため、解除が外すのはこの列だけであり、併せて満杯になっている
-- スレッドはその理由で閉じたままになる。
ALTER TABLE threads ADD COLUMN locked_at DATETIME;

-- unpublished_atという名前はそれが行うことに由来するもので、deleted_atとは別の行き先
-- を表す。deleted_atはパージへ向かう行の印だが、非公開のスレッドや投稿はタイトルと本文を
-- 保ち続ける。印は対象を隠すものであり、印を外せば戻る。
ALTER TABLE threads ADD COLUMN unpublished_at DATETIME;

ALTER TABLE posts ADD COLUMN unpublished_at DATETIME;

-- suspended_atをusers.deleted_atとは別に置くのは、停止と退会がアカウントを別の場所に
-- 置くためである。停止されたアカウントは身元を保ち (emailもatnameもそのままである)、
-- 止まるのは活動だけだが、退会したアカウントは匿名化されパージへ向かう。1つの列が両方を
-- 持てば、停止の解除と退会の取り消しを区別できなくなる。
ALTER TABLE users ADD COLUMN suspended_at DATETIME;

-- moderation_logsは、それらの状態を生んだ操作を記録する。管理者が何を行ったかを、
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
