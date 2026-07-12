-- migrate:up

-- user_two_factor_auths holds a user's TOTP-based two-factor authentication
-- setting, kept in its own table (one row per user) rather than on users so the
-- 2FA credential material stays separate from identity, the same way
-- user_passwords isolates the password credential. A row is created when the
-- user starts enrolling (secret issued, enabled = false) and is flipped to
-- enabled once they confirm a TOTP code; recovery_codes then holds the one-time
-- backup codes and is emptied of a code as it is used.
--
-- secret and recovery_codes are stored in plaintext: Groobb has no encryption
-- key management yet, and the parent 2FA plan follows Wikino, which also stores
-- them in plaintext. Protection therefore relies on database access control.
-- recovery_codes is NOT NULL DEFAULT '{}' so an enrolling (not-yet-enabled) row
-- carries an empty array rather than NULL, keeping the Go side free of nil array
-- handling.
--
-- The user_id FK is UNIQUE (at most one 2FA setting per user) and ON DELETE
-- CASCADE: the setting is pure dependent data with no lifecycle of its own and
-- nothing to clean up outside its own row, so it must disappear with the user.
-- CASCADE guarantees this at the database level even if future deletion code
-- forgets to remove it first, and the UNIQUE user_id index keeps the cascade
-- from scanning the whole table.
--
-- [Ja] user_two_factor_auths はユーザーの TOTP による 2 段階認証設定を保持し、users
-- 本体ではなく専用テーブル (ユーザーあたり 1 行) に置くことで、2FA の資格情報を身元と
-- 分離する (user_passwords がパスワード資格情報を分離するのと同じ)。行はユーザーが
-- 登録を開始した時点で作成され (secret 発行、enabled = false)、TOTP コードの確認後に
-- enabled へ変わる。recovery_codes はその後 1 回使い切りのバックアップコードを保持し、
-- 使用したコードは配列から削除される。
--
-- secret と recovery_codes は平文で保存する。Groobb には暗号鍵管理の基盤がまだ無く、
-- 親の 2FA 計画は平文保存の Wikino を踏襲するため。保護は DB のアクセス制御に依存する。
-- recovery_codes を NOT NULL DEFAULT '{}' とするのは、登録中の (未有効化の) 行が NULL
-- ではなく空配列を持つようにし、Go 側で nil 配列を扱わずに済ませるためである。
--
-- user_id の外部キーは UNIQUE (ユーザーあたり高々 1 つの 2FA 設定) かつ ON DELETE
-- CASCADE とする。この設定は独立したライフサイクルを持たず、自身の行の外に後始末すべき
-- ものもない純粋な従属データのため、ユーザーと一緒に消えるべきである。CASCADE なら将来の
-- 削除コードが先に消し忘れても DB レベルで整合性が保証され、UNIQUE な user_id インデックス
-- によりカスケードがテーブル全体を走査することも避けられる。
CREATE TABLE user_two_factor_auths (
    id uuid DEFAULT public.generate_ulid() NOT NULL PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    secret VARCHAR NOT NULL,
    enabled boolean NOT NULL DEFAULT false,
    enabled_at TIMESTAMP WITH TIME ZONE,
    recovery_codes text[] NOT NULL DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE (user_id)
);

-- migrate:down

DROP TABLE IF EXISTS user_two_factor_auths;
