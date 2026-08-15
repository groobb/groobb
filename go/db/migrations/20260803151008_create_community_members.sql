-- migrate:up

-- community_members is the membership that ties a user to a community. Per
-- ADR 0003 it carries no handle of its own: the globally unique atname lives on
-- users, so a membership records only that a user belongs to a community, with
-- the roles they hold there kept in community_member_roles.
--
-- Both foreign keys are ON DELETE CASCADE: a membership belongs to exactly one
-- community and one user, has no lifecycle of its own and nothing to clean up
-- outside its own row, so it must disappear with either parent. CASCADE
-- guarantees this at the database level even if future deletion code forgets to
-- remove the memberships first.
--
-- UNIQUE (community_id, user_id) keeps a user from joining the same community
-- twice. Having community_id as its leading column, it doubles as the index that
-- lists a community's members and that keeps the cascade from scanning the whole
-- table when a community is deleted.
--
-- UNIQUE (community_id, id) is redundant on its own, since id is already the
-- primary key. It exists so that community_member_roles can carry community_id in
-- a composite foreign key: PostgreSQL only lets a foreign key reference columns
-- covered by a unique constraint. Routing that reference through community_id is
-- what makes the database able to reject assigning a role of another community.
--
-- index_community_members_on_user_id backs the user_id foreign key so that
-- deleting a user does not scan the whole table, and it also serves the lookup of
-- the communities a given user belongs to. community_id needs no index of its
-- own: it leads both UNIQUE constraints above.
--
-- [Ja] community_members はユーザーをコミュニティに結び付けるメンバーシップである。
-- ADR 0003 のとおり自身のハンドルは持たない。グローバルに一意な atname は users が
-- 持つため、メンバーシップはユーザーがそのコミュニティに所属することだけを記録し、
-- そこで持つロールは community_member_roles が保持する。
--
-- 2 本の外部キーはいずれも ON DELETE CASCADE とする。メンバーシップはちょうど 1 つの
-- コミュニティと 1 人のユーザーに属し、独立したライフサイクルを持たず、自身の行の外に
-- 後始末すべきものもないため、どちらの親が消えても一緒に消えるべきである。CASCADE なら
-- 将来の削除コードが先にメンバーシップを消し忘れても DB レベルで整合性が保証される。
--
-- UNIQUE (community_id, user_id) は同じユーザーが同じコミュニティへ二重に所属することを
-- 防ぐ。community_id が先頭カラムであるため、コミュニティのメンバー一覧を引く索引と、
-- コミュニティ削除時にカスケードがテーブル全体を走査するのを防ぐ索引を兼ねる。
--
-- UNIQUE (community_id, id) は id が既に主キーであるため、それ自体は冗長である。これは
-- community_member_roles が複合外部キーに community_id を含められるようにするために置く。
-- PostgreSQL は一意制約で覆われたカラムの組しか外部キーの参照先にできないためである。
-- 参照を community_id 経由にすることで初めて、別のコミュニティのロールの割当を DB が
-- 拒否できるようになる。
--
-- index_community_members_on_user_id は user_id の外部キーを裏付ける索引で、ユーザー削除
-- 時にテーブル全体を走査しないようにし、あわせて「あるユーザーが所属するコミュニティ」の
-- 逆引きにも効く。community_id は上記 2 つの UNIQUE 制約の先頭カラムであるため、単独の
-- 索引は不要である。
CREATE TABLE community_members (
    id uuid DEFAULT public.generate_ulid() NOT NULL PRIMARY KEY,
    community_id uuid NOT NULL REFERENCES communities (id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE (community_id, user_id),
    UNIQUE (community_id, id)
);

CREATE INDEX index_community_members_on_user_id ON community_members (user_id);

-- migrate:down

DROP INDEX IF EXISTS index_community_members_on_user_id;

DROP TABLE IF EXISTS community_members;
