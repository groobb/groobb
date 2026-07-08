-- migrate:up

-- Add a globally unique atname (@handle) to users. atname is the stable,
-- human-facing identifier of who a poster is; it underpins impersonation
-- resistance, future mentions, and user-facing URLs (ADR 0003). It is citext, so
-- letter-case variants (Foo vs foo) collapse to one handle just as they do for
-- email, and the UNIQUE constraint enforces global uniqueness as the last line
-- of defense behind the application-level check. It is added NOT NULL without a
-- default because Groobb is pre-release with no production users; reset the
-- local database if a dev row already exists.
--
-- [Ja] users にグローバルに一意な atname (@ハンドル) を追加する。atname は「投稿者が
-- 何者か」を示す安定した対人向けの識別子であり、なりすまし防止・将来のメンション・
-- ユーザーを指す URL の土台になる (ADR 0003)。email 列と同様に citext とし、大文字小文字
-- 違い (Foo と foo) を 1 つのハンドルに畳み込む。UNIQUE 制約はアプリ層チェックの背後の
-- 最終防衛線としてグローバルな一意性を強制する。既定値なしの NOT NULL で追加するのは
-- Groobb がリリース前で本番ユーザーが無いためで、ローカルに既存行がある場合は DB を
-- リセットする。
ALTER TABLE users ADD COLUMN atname public.citext NOT NULL;
ALTER TABLE users ADD CONSTRAINT users_atname_key UNIQUE (atname);

-- migrate:down

ALTER TABLE users DROP CONSTRAINT IF EXISTS users_atname_key;
ALTER TABLE users DROP COLUMN IF EXISTS atname;
