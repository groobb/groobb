-- migrate:up

-- communities is the container that boards and their posts hang off of
-- (ADR 0002). name is the human-facing display name, and identifier is the URL
-- identifier that addresses the community in the short path /c/{identifier}.
-- identifier is citext + UNIQUE so that letter-case variants (Foo vs foo)
-- collapse to one community just as they do for users.email and users.atname,
-- and the UNIQUE constraint is the last line of defense behind the
-- application-level uniqueness check. Length limits are validated in the
-- application, so neither column carries a length modifier.
--
-- [Ja] communities は掲示板とその投稿がぶら下がるコンテナ (ADR 0002)。name は対人向けの
-- 表示名、identifier は短縮パス /c/{identifier} でコミュニティを指す URL 識別子である。
-- identifier は users.email や users.atname と同様に citext + UNIQUE とし、大文字小文字
-- 違い (Foo と foo) を 1 つのコミュニティに畳み込む。UNIQUE 制約はアプリ層の一意性チェック
-- の背後にある最終防衛線として働く。長さ制限はアプリ側でバリデーションするため、どちらの
-- カラムにも長さ指定は付けない。
CREATE TABLE communities (
    id uuid DEFAULT public.generate_ulid() NOT NULL PRIMARY KEY,
    name VARCHAR NOT NULL,
    identifier public.citext NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE (identifier)
);

-- migrate:down

DROP TABLE IF EXISTS communities;
