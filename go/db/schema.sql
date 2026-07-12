
-- Dumped from database version 18.3 (Debian 18.3-1.pgdg13+1)
-- Dumped by pg_dump version 18.4 (Debian 18.4-1.pgdg13+1)

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET transaction_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: citext; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS citext WITH SCHEMA public;


--
-- Name: EXTENSION citext; Type: COMMENT; Schema: -; Owner: -
--

COMMENT ON EXTENSION citext IS 'data type for case-insensitive character strings';


--
-- Name: pgcrypto; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS pgcrypto WITH SCHEMA public;


--
-- Name: EXTENSION pgcrypto; Type: COMMENT; Schema: -; Owner: -
--

COMMENT ON EXTENSION pgcrypto IS 'cryptographic functions';


--
-- Name: river_job_state; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.river_job_state AS ENUM (
    'available',
    'cancelled',
    'completed',
    'discarded',
    'pending',
    'retryable',
    'running',
    'scheduled'
);


--
-- Name: generate_ulid(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.generate_ulid() RETURNS uuid
    LANGUAGE sql
    AS $$
  SELECT (lpad(to_hex(floor(extract(epoch FROM clock_timestamp()) * 1000)::bigint), 12, '0') || encode(gen_random_bytes(10), 'hex'))::uuid;
$$;


--
-- Name: river_job_state_in_bitmask(bit, public.river_job_state); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.river_job_state_in_bitmask(bitmask bit, state public.river_job_state) RETURNS boolean
    LANGUAGE sql IMMUTABLE
    AS $$
    SELECT CASE state
        WHEN 'available' THEN get_bit(bitmask, 7)
        WHEN 'cancelled' THEN get_bit(bitmask, 6)
        WHEN 'completed' THEN get_bit(bitmask, 5)
        WHEN 'discarded' THEN get_bit(bitmask, 4)
        WHEN 'pending'   THEN get_bit(bitmask, 3)
        WHEN 'retryable' THEN get_bit(bitmask, 2)
        WHEN 'running'   THEN get_bit(bitmask, 1)
        WHEN 'scheduled' THEN get_bit(bitmask, 0)
        ELSE 0
    END = 1;
$$;


SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: email_confirmations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.email_confirmations (
    id uuid DEFAULT public.generate_ulid() NOT NULL,
    email public.citext NOT NULL,
    event character varying NOT NULL,
    code character varying NOT NULL,
    started_at timestamp with time zone DEFAULT now() NOT NULL,
    succeeded_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    failed_attempts_count integer DEFAULT 0 NOT NULL,
    user_id uuid
);


--
-- Name: password_reset_tokens; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.password_reset_tokens (
    id uuid DEFAULT public.generate_ulid() NOT NULL,
    user_id uuid NOT NULL,
    token_digest character varying NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    used_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: river_job; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.river_job (
    id bigint NOT NULL,
    state public.river_job_state DEFAULT 'available'::public.river_job_state NOT NULL,
    attempt smallint DEFAULT 0 NOT NULL,
    max_attempts smallint DEFAULT 25 NOT NULL,
    attempted_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    finalized_at timestamp with time zone,
    scheduled_at timestamp with time zone DEFAULT now() NOT NULL,
    priority smallint DEFAULT 1 NOT NULL,
    args jsonb NOT NULL,
    attempted_by text[],
    errors jsonb[],
    kind text NOT NULL,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    queue text DEFAULT 'default'::text NOT NULL,
    tags character varying(255)[] DEFAULT '{}'::character varying[] NOT NULL,
    unique_key bytea,
    unique_states bit(8),
    CONSTRAINT finalized_or_finalized_at_null CHECK ((((finalized_at IS NULL) AND (state <> ALL (ARRAY['cancelled'::public.river_job_state, 'completed'::public.river_job_state, 'discarded'::public.river_job_state]))) OR ((finalized_at IS NOT NULL) AND (state = ANY (ARRAY['cancelled'::public.river_job_state, 'completed'::public.river_job_state, 'discarded'::public.river_job_state]))))),
    CONSTRAINT kind_length CHECK (((char_length(kind) > 0) AND (char_length(kind) < 128))),
    CONSTRAINT max_attempts_is_positive CHECK ((max_attempts > 0)),
    CONSTRAINT priority_in_range CHECK (((priority >= 1) AND (priority <= 4))),
    CONSTRAINT queue_length CHECK (((char_length(queue) > 0) AND (char_length(queue) < 128)))
);


--
-- Name: river_job_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.river_job_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: river_job_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.river_job_id_seq OWNED BY public.river_job.id;


--
-- Name: river_leader; Type: TABLE; Schema: public; Owner: -
--

CREATE UNLOGGED TABLE public.river_leader (
    elected_at timestamp with time zone NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    leader_id text NOT NULL,
    name text DEFAULT 'default'::text NOT NULL,
    CONSTRAINT leader_id_length CHECK (((char_length(leader_id) > 0) AND (char_length(leader_id) < 128))),
    CONSTRAINT name_length CHECK ((name = 'default'::text))
);


--
-- Name: river_migration; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.river_migration (
    line text NOT NULL,
    version bigint NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT line_length CHECK (((char_length(line) > 0) AND (char_length(line) < 128))),
    CONSTRAINT version_gte_1 CHECK ((version >= 1))
);


--
-- Name: river_notification; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.river_notification (
    id bigint NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    payload text NOT NULL,
    topic text NOT NULL,
    CONSTRAINT topic_length CHECK (((length(topic) > 0) AND (length(topic) < 128)))
);


--
-- Name: river_notification_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.river_notification_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: river_notification_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.river_notification_id_seq OWNED BY public.river_notification.id;


--
-- Name: river_queue; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.river_queue (
    name text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    paused_at timestamp with time zone,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


--
-- Name: schema_migrations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.schema_migrations (
    version character varying NOT NULL
);


--
-- Name: user_passwords; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_passwords (
    id uuid DEFAULT public.generate_ulid() NOT NULL,
    user_id uuid NOT NULL,
    password_digest character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: user_sessions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_sessions (
    id uuid DEFAULT public.generate_ulid() NOT NULL,
    user_id uuid NOT NULL,
    token character varying NOT NULL,
    ip_address character varying NOT NULL,
    user_agent character varying NOT NULL,
    signed_in_at timestamp with time zone DEFAULT now() NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: user_two_factor_auths; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_two_factor_auths (
    id uuid DEFAULT public.generate_ulid() NOT NULL,
    user_id uuid NOT NULL,
    secret character varying NOT NULL,
    enabled boolean DEFAULT false NOT NULL,
    enabled_at timestamp with time zone,
    recovery_codes text[] DEFAULT '{}'::text[] NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: users; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.users (
    id uuid DEFAULT public.generate_ulid() NOT NULL,
    email public.citext NOT NULL,
    locale character varying NOT NULL,
    time_zone character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    atname public.citext NOT NULL,
    deleted_at timestamp with time zone
);


--
-- Name: river_job id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.river_job ALTER COLUMN id SET DEFAULT nextval('public.river_job_id_seq'::regclass);


--
-- Name: river_notification id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.river_notification ALTER COLUMN id SET DEFAULT nextval('public.river_notification_id_seq'::regclass);


--
-- Name: email_confirmations email_confirmations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.email_confirmations
    ADD CONSTRAINT email_confirmations_pkey PRIMARY KEY (id);


--
-- Name: password_reset_tokens password_reset_tokens_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.password_reset_tokens
    ADD CONSTRAINT password_reset_tokens_pkey PRIMARY KEY (id);


--
-- Name: password_reset_tokens password_reset_tokens_token_digest_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.password_reset_tokens
    ADD CONSTRAINT password_reset_tokens_token_digest_key UNIQUE (token_digest);


--
-- Name: river_job river_job_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.river_job
    ADD CONSTRAINT river_job_pkey PRIMARY KEY (id);


--
-- Name: river_leader river_leader_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.river_leader
    ADD CONSTRAINT river_leader_pkey PRIMARY KEY (name);


--
-- Name: river_migration river_migration_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.river_migration
    ADD CONSTRAINT river_migration_pkey PRIMARY KEY (line, version);


--
-- Name: river_notification river_notification_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.river_notification
    ADD CONSTRAINT river_notification_pkey PRIMARY KEY (id);


--
-- Name: river_queue river_queue_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.river_queue
    ADD CONSTRAINT river_queue_pkey PRIMARY KEY (name);


--
-- Name: schema_migrations schema_migrations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.schema_migrations
    ADD CONSTRAINT schema_migrations_pkey PRIMARY KEY (version);


--
-- Name: user_passwords user_passwords_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_passwords
    ADD CONSTRAINT user_passwords_pkey PRIMARY KEY (id);


--
-- Name: user_passwords user_passwords_user_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_passwords
    ADD CONSTRAINT user_passwords_user_id_key UNIQUE (user_id);


--
-- Name: user_sessions user_sessions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_sessions
    ADD CONSTRAINT user_sessions_pkey PRIMARY KEY (id);


--
-- Name: user_sessions user_sessions_token_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_sessions
    ADD CONSTRAINT user_sessions_token_key UNIQUE (token);


--
-- Name: user_two_factor_auths user_two_factor_auths_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_two_factor_auths
    ADD CONSTRAINT user_two_factor_auths_pkey PRIMARY KEY (id);


--
-- Name: user_two_factor_auths user_two_factor_auths_user_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_two_factor_auths
    ADD CONSTRAINT user_two_factor_auths_user_id_key UNIQUE (user_id);


--
-- Name: users users_atname_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_atname_key UNIQUE (atname);


--
-- Name: users users_email_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_email_key UNIQUE (email);


--
-- Name: users users_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);


--
-- Name: index_email_confirmations_on_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_email_confirmations_on_user_id ON public.email_confirmations USING btree (user_id);


--
-- Name: index_password_reset_tokens_on_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_password_reset_tokens_on_user_id ON public.password_reset_tokens USING btree (user_id);


--
-- Name: index_users_on_deleted_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX index_users_on_deleted_at ON public.users USING btree (deleted_at) WHERE (deleted_at IS NOT NULL);


--
-- Name: river_job_args_index; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX river_job_args_index ON public.river_job USING gin (args);


--
-- Name: river_job_kind; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX river_job_kind ON public.river_job USING btree (kind);


--
-- Name: river_job_metadata_index; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX river_job_metadata_index ON public.river_job USING gin (metadata);


--
-- Name: river_job_prioritized_fetching_index; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX river_job_prioritized_fetching_index ON public.river_job USING btree (state, queue, priority, scheduled_at, id);


--
-- Name: river_job_state_and_finalized_at_index; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX river_job_state_and_finalized_at_index ON public.river_job USING btree (state, finalized_at) WHERE (finalized_at IS NOT NULL);


--
-- Name: river_job_unique_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX river_job_unique_idx ON public.river_job USING btree (unique_key) WHERE ((unique_key IS NOT NULL) AND (unique_states IS NOT NULL) AND public.river_job_state_in_bitmask(unique_states, state));


--
-- Name: river_notification_created_at_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX river_notification_created_at_idx ON public.river_notification USING btree (created_at);


--
-- Name: river_notification_topic_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX river_notification_topic_id_idx ON public.river_notification USING btree (topic, id);


--
-- Name: user_sessions_user_id_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX user_sessions_user_id_idx ON public.user_sessions USING btree (user_id);


--
-- Name: email_confirmations email_confirmations_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.email_confirmations
    ADD CONSTRAINT email_confirmations_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: password_reset_tokens password_reset_tokens_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.password_reset_tokens
    ADD CONSTRAINT password_reset_tokens_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: user_passwords user_passwords_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_passwords
    ADD CONSTRAINT user_passwords_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: user_sessions user_sessions_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_sessions
    ADD CONSTRAINT user_sessions_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: user_two_factor_auths user_two_factor_auths_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_two_factor_auths
    ADD CONSTRAINT user_two_factor_auths_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- PostgreSQL database dump complete
--



--
-- Dbmate schema migrations
--

INSERT INTO public.schema_migrations (version) VALUES
    ('20260614160555'),
    ('20260614160556'),
    ('20260615021215'),
    ('20260615120000'),
    ('20260616014134'),
    ('20260616023735'),
    ('20260616152453'),
    ('20260617054258'),
    ('20260704182204'),
    ('20260708015309'),
    ('20260709011939'),
    ('20260709160424'),
    ('20260710063911');
