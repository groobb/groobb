-- +goose Up

-- The admin role is the one role an instance starts with, and the only one the
-- application names in its own code. It carries community:admin alone, which
-- implies every scope the vocabulary holds now and every scope added later, so
-- the plans that introduce new administrative operations give those operations
-- scopes without coming back to rewrite this row.
--
-- The row is created by a migration rather than by the application at startup
-- because an instance that never reaches its first sign-in still has to be able
-- to be given an administrator, and whatever assigns the role has to find it
-- already there. Inserting it here also makes the row part of the schema every
-- environment shares, instead of something each database acquires at a
-- different moment.
--
-- The name is what everything reaches this role by, since the id belongs to
-- whichever database the migration ran against.
--
-- [Ja] admin ロールは、インスタンスが最初から持つ唯一のロールであり、アプリケーションが
-- 自身のコードで名指す唯一のロールでもある。持つスコープは community:admin だけで、これは
-- 現在の語彙のすべてのスコープと、後から足されるスコープをも含意する。そのため、新しい
-- 管理操作を導入する計画は、その操作にスコープを与えるだけでよく、この行を書き換えに戻って
-- くる必要はない。
--
-- 行をアプリケーションの起動時ではなくマイグレーションで作るのは、最初のサインインにすら
-- 到達していないインスタンスでも管理者を立てられなければならないためである。ロールを
-- 割り当てる側は、それが既にそこにあることを前提にできる。ここで挿入することは、この行を
-- 各データベースがそれぞれ別の時点で手に入れるものではなく、どの環境も共有するスキーマの
-- 一部にすることでもある。
--
-- id はマイグレーションを適用したデータベースごとのものであるため、すべてはこのロールに
-- 名前で辿り着く。
INSERT INTO roles (name, scopes) VALUES ('admin', '["community:admin"]');

-- +goose Down

DELETE FROM roles WHERE name = 'admin';
