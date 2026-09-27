-- +goose Up
-- AI email assistant foundation: per-user opt-in settings, user-managed
-- categorization rules, and applied category labels + extracted features per
-- message (features stored, not just verdicts, to keep the design open to a
-- trained classifier later).

CREATE TABLE IF NOT EXISTS assistant_settings (
    user_id            uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    -- Master opt-in. Defaults to false: the assistant is off unless the user
    -- explicitly enables it.
    enabled            boolean NOT NULL DEFAULT false,
    -- Feature-level opt-ins (all gated behind the master enabled flag).
    summarization_enabled   boolean NOT NULL DEFAULT true,
    categorization_enabled  boolean NOT NULL DEFAULT true,
    compose_assist_enabled  boolean NOT NULL DEFAULT true,
    -- Provider selection. "local" is the default offline provider; other
    -- values are only honored when an external provider is configured by the
    -- operator. No mail content leaves the server with "local".
    provider           text NOT NULL DEFAULT 'local',
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT assistant_settings_provider_nonempty CHECK (length(trim(provider)) > 0)
);

CREATE TABLE IF NOT EXISTS assistant_categorization_rules (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    category    text NOT NULL,
    -- Which message field the keyword is matched against.
    field       text NOT NULL DEFAULT 'any',
    -- How the keyword is compared.
    match_type  text NOT NULL DEFAULT 'contains',
    keyword     text NOT NULL,
    priority    integer NOT NULL DEFAULT 0,
    weight      double precision NOT NULL DEFAULT 1,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT assistant_rule_category_nonempty CHECK (length(trim(category)) > 0),
    CONSTRAINT assistant_rule_keyword_nonempty CHECK (length(trim(keyword)) > 0),
    CONSTRAINT assistant_rule_field CHECK (field IN ('from', 'subject', 'body', 'to', 'any')),
    CONSTRAINT assistant_rule_match_type CHECK (match_type IN ('contains', 'prefix', 'suffix', 'exact'))
);

CREATE INDEX IF NOT EXISTS idx_assistant_rules_user
    ON assistant_categorization_rules(user_id, priority DESC, created_at ASC);

-- Applied labels + extracted features per message. One row per message that
-- the categorizer has processed. features is a classifier-ready JSON blob.
CREATE TABLE IF NOT EXISTS assistant_message_labels (
    message_id    uuid PRIMARY KEY REFERENCES messages(id) ON DELETE CASCADE,
    user_id       uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    category      text NOT NULL DEFAULT '',
    confidence    double precision NOT NULL DEFAULT 0,
    matched_rule_ids uuid[] NOT NULL DEFAULT '{}',
    features      jsonb NOT NULL DEFAULT '{}'::jsonb,
    applied_at    timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_assistant_labels_user_category
    ON assistant_message_labels(user_id, category, applied_at DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_assistant_labels_user_category;
DROP TABLE IF EXISTS assistant_message_labels;
DROP INDEX IF EXISTS idx_assistant_rules_user;
DROP TABLE IF EXISTS assistant_categorization_rules;
DROP TABLE IF EXISTS assistant_settings;
