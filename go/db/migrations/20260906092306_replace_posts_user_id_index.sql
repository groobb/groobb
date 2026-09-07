-- +goose Up

-- The interval between one person's posts is measured from a single row: the
-- latest post they wrote. index_posts_on_user_id leads to that author's posts
-- but says nothing about which of them is the latest, so the lookup has to
-- collect a whole posting history and sort it to keep one row -- work that grows
-- with how much the person has written, on the path every post takes.
--
-- Carrying created_at in the index leaves the first row visited as the answer.
-- id follows it in the same direction so that posts sharing a timestamp still
-- have one of them as the latest: SQLite stores a timestamp to the millisecond,
-- and two posts written within one is what a caller unable to break the tie
-- would read as either row.
--
-- The old index is dropped rather than kept alongside. The new one leads with
-- the same column, so the lookups the old one served -- ON DELETE SET NULL
-- reaching an account's posts, above all -- follow the new one just as directly,
-- and keeping both would leave every post to write two indexes instead of one.
--
-- [Ja] ある人の投稿の間隔は 1 行から測る。その人が最後に書いた投稿である。
-- index_posts_on_user_id はその作者の投稿までは導くが、そのどれが最新かは何も語らない
-- ため、引き当ては投稿履歴を丸ごと集めて並べ替え、1 行だけを残すことになる。この仕事は
-- その人が書いた量に比例して増え、しかもすべての投稿が通る経路の上にある。
--
-- created_at を索引に持たせると、最初に訪れる行がそのまま答えになる。id を同じ向きで
-- 続けるのは、時刻が同じ投稿でもどちらかが最新になるようにするためである。SQLite が
-- 保持する時刻はミリ秒までであり、その中に収まる 2 件の投稿は、同着を解けない呼び出し元
-- からはどちらの行にも読めてしまう。
--
-- 古い索引は残さず削除する。新しい索引は同じ列で始まるため、古い索引が担っていた
-- 引き当て (何より ON DELETE SET NULL がアカウントの投稿へ届くこと) は新しい索引を
-- 同じだけ直接たどる。両方を残せば、投稿を書き込むたびに 1 つで済む索引の更新が 2 つに
-- なる。
CREATE INDEX index_posts_on_user_id_and_created_at_and_id ON posts (user_id, created_at DESC, id DESC);

DROP INDEX index_posts_on_user_id;

-- +goose Down

CREATE INDEX index_posts_on_user_id ON posts (user_id);

DROP INDEX index_posts_on_user_id_and_created_at_and_id;
