ALTER TABLE task_defs ADD COLUMN metric text NOT NULL DEFAULT 'vote';

UPDATE task_defs SET metric = CASE code
    WHEN 'daily_vote' THEN 'vote'
    WHEN 'daily_post' THEN 'post'
    WHEN 'daily_page' THEN 'page'
    WHEN 'monthly_uv30' THEN 'uv30'
    WHEN 'monthly_dv30' THEN 'dv30'
    WHEN 'monthly_comments20' THEN 'comments'
    ELSE metric END;

ALTER TABLE task_defs ADD CONSTRAINT task_defs_metric_check
    CHECK (metric IN ('vote', 'post', 'page', 'uv30', 'dv30', 'comments'));
ALTER TABLE task_defs ADD CONSTRAINT task_defs_kind_check
    CHECK (kind IN ('daily', 'monthly'));
ALTER TABLE task_defs ADD CONSTRAINT task_defs_reward_check CHECK (reward >= 0);
ALTER TABLE task_defs ADD CONSTRAINT task_defs_target_check CHECK (target >= 1);

CREATE TABLE tips (
    id         bigserial PRIMARY KEY,
    from_user  bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    article_id bigint NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
    amount     bigint NOT NULL CHECK (amount > 0),
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX tips_article_idx ON tips (article_id);
CREATE INDEX tips_from_idx ON tips (from_user, created_at DESC);

ALTER TABLE articles ADD COLUMN author_ids bigint[] NOT NULL DEFAULT '{}';
ALTER TABLE articles ADD COLUMN author_names text[] NOT NULL DEFAULT '{}';
