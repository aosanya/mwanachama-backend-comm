-- schema.sql is comm's own squashed, comm_-prefixed schema fixture, used
-- ONLY by this module's own -tags postgres scratch-DB tests
-- (postgres_scratch_test.go). It is NOT a production migration — the
-- production rename lives in mwanachama-backend-api-gateway's
-- internal/store/postgres/migrations/ as a single forward
-- ALTER TABLE ... RENAME migration over the live tables this schema mirrors.
--
-- Consolidated from the gateway's historical chat/directmessage/moderation
-- migrations (000005, 000006, 000014, 000015, 000021, 000027, 000028,
-- 000039-000045) into the final column shape each table actually holds
-- today, with two deliberate divergences from the production shape:
--
--   1. The four moderation tables' `REFERENCES member(id)` foreign keys are
--      dropped here — this scratch DB has no `member` table, and the FK
--      backs no Go-level invariant these tests check (member existence is a
--      chapter-scoped HTTP-layer concern). Extends the same reasoning the
--      original migration already applies to every `chapter_id` column
--      ("a different store's fixture in this port").
--   2. comm_test_act_log is NOT a production table at all — it exists only
--      so postgres_scratch_test.go's testTxActWriter can prove moderation's
--      act-log write happens in the same transaction as the moderation row,
--      the way the gateway's real chapter_act_log_entry does in production.

CREATE SEQUENCE IF NOT EXISTS comm_chat_thread_seq;
CREATE SEQUENCE IF NOT EXISTS comm_chat_message_seq;

CREATE TABLE IF NOT EXISTS comm_chat_thread (
    id         text PRIMARY KEY DEFAULT ('cthread-' || nextval('comm_chat_thread_seq')),
    chapter_id text NOT NULL,
    tag_path   text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (chapter_id, tag_path)
);

CREATE TABLE IF NOT EXISTS comm_chat_message (
    id         text PRIMARY KEY DEFAULT ('msg-' || nextval('comm_chat_message_seq')),
    chapter_id text NOT NULL,
    thread_id  text,
    author_id  text NOT NULL,
    body       text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS comm_chat_message_chapter_idx ON comm_chat_message (chapter_id, created_at);
CREATE INDEX IF NOT EXISTS comm_chat_message_thread_idx ON comm_chat_message (thread_id, created_at);

CREATE SEQUENCE IF NOT EXISTS comm_dm_thread_seq;
CREATE SEQUENCE IF NOT EXISTS comm_dm_message_seq;
CREATE SEQUENCE IF NOT EXISTS comm_dm_device_key_seq;

-- Final shape after 000006 -> 000039 -> 000041 -> 000042 -> 000043 -> 000044
-- -> 000045; opened_via_address (text) was added by 000039 and dropped again
-- by 000041 in favour of opened_via_address_hash, so it does not appear here.
CREATE TABLE IF NOT EXISTS comm_dm_thread (
    id                        text PRIMARY KEY DEFAULT ('dm-' || nextval('comm_dm_thread_seq')),
    title                     text,
    created_by                text NOT NULL,
    created_at                timestamptz NOT NULL DEFAULT now(),
    opened_via_address_hash   bytea,
    opened_via_address_owner  text,
    opened_via_address_index  integer,
    sent_from_address_owner   text,
    sent_from_address_index   integer,
    sent_from_address_sealed  jsonb,
    message_ttl_seconds       integer
);

CREATE TABLE IF NOT EXISTS comm_dm_participant (
    thread_id  text NOT NULL REFERENCES comm_dm_thread(id) ON DELETE CASCADE,
    member_id  text NOT NULL,
    state      text NOT NULL,
    is_admin   boolean NOT NULL DEFAULT false,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (thread_id, member_id)
);
CREATE INDEX IF NOT EXISTS comm_dm_participant_member_idx ON comm_dm_participant (member_id, state);

CREATE TABLE IF NOT EXISTS comm_dm_message (
    id                   text PRIMARY KEY DEFAULT ('dmmsg-' || nextval('comm_dm_message_seq')),
    thread_id            text NOT NULL REFERENCES comm_dm_thread(id) ON DELETE CASCADE,
    sender_id            text NOT NULL,
    sender_device_key_id text NOT NULL,
    payload_ciphertext   text NOT NULL,
    per_recipient_keys   jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at           timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS comm_dm_message_thread_idx ON comm_dm_message (thread_id, created_at);

-- Final shape after 000006 -> 000015 (data-only purge, no shape change) ->
-- 000021 -> 000027.
CREATE TABLE IF NOT EXISTS comm_dm_device_key (
    member_id    text NOT NULL,
    key_id       text NOT NULL,
    public_key   text NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    published_by text,
    device_id    text,
    retired_at   timestamptz,
    PRIMARY KEY (member_id, key_id)
);
CREATE INDEX IF NOT EXISTS comm_dm_device_key_live_idx
    ON comm_dm_device_key (member_id) WHERE retired_at IS NULL;
CREATE INDEX IF NOT EXISTS comm_dm_device_key_published_by_idx
    ON comm_dm_device_key (published_by);
CREATE INDEX IF NOT EXISTS comm_dm_device_key_device_live_idx
    ON comm_dm_device_key (device_id) WHERE retired_at IS NULL;

CREATE TABLE IF NOT EXISTS comm_dm_message_reaction (
    message_id text NOT NULL REFERENCES comm_dm_message(id) ON DELETE CASCADE,
    member_id  text NOT NULL,
    emoji      text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (message_id, member_id)
);
CREATE INDEX IF NOT EXISTS comm_dm_message_reaction_message_idx
    ON comm_dm_message_reaction (message_id);

CREATE SEQUENCE IF NOT EXISTS comm_message_report_seq;
CREATE SEQUENCE IF NOT EXISTS comm_message_removal_seq;
CREATE SEQUENCE IF NOT EXISTS comm_removal_dispute_seq;
CREATE SEQUENCE IF NOT EXISTS comm_message_report_dismissal_seq;

-- member(id) FKs dropped in this fixture only — see file header note 1.
CREATE TABLE IF NOT EXISTS comm_message_report (
    id                   text PRIMARY KEY DEFAULT ('modreport-' || nextval('comm_message_report_seq')),
    message_id           text NOT NULL REFERENCES comm_chat_message(id),
    chapter_id           text NOT NULL,
    reported_by          text NOT NULL,
    reporter_role_class  text NOT NULL,
    reason               text NOT NULL CHECK (reason IN ('threats', 'abuse', 'false_claim', 'spam')),
    note                 text,
    excerpt              text NOT NULL,
    reported_at          timestamptz NOT NULL DEFAULT now(),
    UNIQUE (message_id, reported_by)
);
CREATE INDEX IF NOT EXISTS comm_message_report_queue_idx ON comm_message_report (chapter_id, message_id);

CREATE TABLE IF NOT EXISTS comm_message_removal (
    id                 text PRIMARY KEY DEFAULT ('modremoval-' || nextval('comm_message_removal_seq')),
    message_id         text NOT NULL UNIQUE REFERENCES comm_chat_message(id),
    chapter_id         text NOT NULL,
    removed_by         text NOT NULL,
    actor_role_class   text NOT NULL,
    reason             text NOT NULL CHECK (reason IN ('threats', 'abuse', 'false_claim')),
    removed_at         timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS comm_removal_dispute (
    id                 text PRIMARY KEY DEFAULT ('moddispute-' || nextval('comm_removal_dispute_seq')),
    removal_id         text NOT NULL UNIQUE REFERENCES comm_message_removal(id),
    raised_by          text NOT NULL,
    statement          text NOT NULL,
    review_chapter_id  text NOT NULL,
    held_since         timestamptz NOT NULL DEFAULT now(),
    raised_at          timestamptz NOT NULL DEFAULT now(),
    state              text NOT NULL DEFAULT 'open' CHECK (state IN ('open', 'reinstated', 'upheld')),
    decided_by         text,
    decided_at         timestamptz,
    CONSTRAINT comm_removal_dispute_decided_pair CHECK ((decided_at IS NULL) = (decided_by IS NULL)),
    CONSTRAINT comm_removal_dispute_open_is_undecided CHECK ((state = 'open') = (decided_at IS NULL))
);

CREATE TABLE IF NOT EXISTS comm_message_report_dismissal (
    id                 text PRIMARY KEY DEFAULT ('moddismissal-' || nextval('comm_message_report_dismissal_seq')),
    message_id         text NOT NULL UNIQUE REFERENCES comm_chat_message(id),
    chapter_id         text NOT NULL,
    dismissed_by       text NOT NULL,
    actor_role_class   text NOT NULL,
    reason             text NOT NULL CHECK (reason IN ('threats', 'abuse', 'false_claim')),
    note               text,
    dismissed_at       timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS comm_message_report_dismissal_chapter_idx
    ON comm_message_report_dismissal (chapter_id, dismissed_at DESC);

-- Test-only stand-in for the gateway's chapter_act_log_entry — see file
-- header note 2. Never created outside this module's own scratch tests.
CREATE TABLE IF NOT EXISTS comm_test_act_log (
    id          bigserial PRIMARY KEY,
    chapter_id  text NOT NULL,
    kind        text NOT NULL,
    actor_id    text,
    subject_id  text,
    occurred_at timestamptz NOT NULL DEFAULT now()
);
