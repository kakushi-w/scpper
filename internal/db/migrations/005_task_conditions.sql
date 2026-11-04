ALTER TABLE task_defs DROP CONSTRAINT task_defs_metric_check;
ALTER TABLE task_defs ADD COLUMN tag text NOT NULL DEFAULT '';
ALTER TABLE task_defs ADD COLUMN category text NOT NULL DEFAULT '';
ALTER TABLE task_defs ADD COLUMN score integer;

UPDATE task_defs SET metric = 'page_score', tag = 'uv至上', score = 30 WHERE metric = 'uv30';
UPDATE task_defs SET metric = 'page_score', tag = 'dv至上', score = -30 WHERE metric = 'dv30';
UPDATE task_defs SET metric = 'comments_received' WHERE metric = 'comments';

ALTER TABLE task_defs ADD CONSTRAINT task_defs_metric_check CHECK (metric IN (
    'vote', 'vote_up', 'vote_down', 'post', 'comment', 'thread_post', 'edit', 'page', 'page_score',
    'comments_received', 'score_received', 'tip_given', 'tip_received', 'gacha', 'trade'));

ALTER TABLE forum_posts ADD COLUMN page_id bigint;
CREATE INDEX forum_posts_page_idx ON forum_posts (page_id) WHERE page_id IS NOT NULL;
