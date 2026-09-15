-- +goose Up

-- ある人の投稿の間隔は1行から測る。その人が最後に書いた投稿である。
-- index_posts_on_user_idはその作者の投稿までは導くが、そのどれが最新かは何も語らない
-- ため、引き当ては投稿履歴を丸ごと集めて並べ替え、1行だけを残すことになる。この仕事は
-- その人が書いた量に比例して増え、しかもすべての投稿が通る経路の上にある。
--
-- created_atを索引に持たせると、最初に訪れる行がそのまま答えになる。idを同じ向きで
-- 続けるのは、時刻が同じ投稿でもどちらかが最新になるようにするためである。SQLiteが
-- 保持する時刻はミリ秒までであり、その中に収まる2件の投稿は、同着を解けない呼び出し元
-- からはどちらの行にも読めてしまう。
--
-- 古い索引は残さず削除する。新しい索引は同じ列で始まるため、古い索引が担っていた
-- 引き当て (何よりON DELETE SET NULLがアカウントの投稿へ届くこと) は新しい索引を
-- 同じだけ直接たどる。両方を残せば、投稿を書き込むたびに1つで済む索引の更新が2つに
-- なる。
CREATE INDEX index_posts_on_user_id_and_created_at_and_id ON posts (user_id, created_at DESC, id DESC);

DROP INDEX index_posts_on_user_id;

-- +goose Down

CREATE INDEX index_posts_on_user_id ON posts (user_id);

DROP INDEX index_posts_on_user_id_and_created_at_and_id;
