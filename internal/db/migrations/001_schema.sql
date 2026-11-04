CREATE EXTENSION IF NOT EXISTS btree_gin;

CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE FUNCTION card_rarity(p_rating double precision) RETURNS text
    LANGUAGE sql IMMUTABLE
    AS $$
    SELECT CASE
        WHEN abs(p_rating) >= 60 THEN 'ur'
        WHEN abs(p_rating) >= 30 THEN 'ssr'
        WHEN abs(p_rating) >= 15 THEN 'sr'
        WHEN abs(p_rating) >= 6  THEN 'r'
        ELSE 'n'
    END;
$$;

CREATE FUNCTION stock_return_shares_on_delete() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    IF OLD.qty <> 0 THEN
        UPDATE stock_maker
        SET inventory = inventory + OLD.qty, updated_at = now()
        WHERE symbol = OLD.symbol;
    END IF;
    RETURN OLD;
END;
$$;

CREATE TABLE articles (
    id bigint NOT NULL,
    wiki text NOT NULL,
    page text NOT NULL,
    page_id bigint,
    title text DEFAULT ''::text NOT NULL,
    url text,
    rating double precision DEFAULT 0 NOT NULL,
    upvotes integer DEFAULT 0 NOT NULL,
    downvotes integer DEFAULT 0 NOT NULL,
    rating_votes integer DEFAULT 0 NOT NULL,
    comments integer DEFAULT 0 NOT NULL,
    author text,
    author_id bigint,
    tags text[] DEFAULT '{}'::text[] NOT NULL,
    created_at timestamp with time zone,
    lastmod timestamp with time zone,
    status text DEFAULT 'active'::text NOT NULL,
    deleted_at timestamp with time zone,
    revision_count integer DEFAULT 0 NOT NULL,
    editor_count integer DEFAULT 0 NOT NULL,
    source text,
    content text,
    source_fetched_at timestamp with time zone,
    source_lastmod timestamp with time zone,
    votes_synced_at timestamp with time zone,
    votes_synced_mark integer,
    revs_synced_at timestamp with time zone,
    revs_synced_mark integer,
    first_seen_at timestamp with time zone DEFAULT now() NOT NULL,
    synced_at timestamp with time zone DEFAULT now() NOT NULL
);

CREATE SEQUENCE articles_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE articles_id_seq OWNED BY articles.id;

CREATE TABLE coin_ledger (
    id bigint NOT NULL,
    user_id bigint NOT NULL,
    delta bigint NOT NULL,
    balance_after bigint NOT NULL,
    reason text NOT NULL,
    ref_type text,
    ref_id bigint,
    memo text,
    operator_id bigint,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);

CREATE SEQUENCE coin_ledger_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE coin_ledger_id_seq OWNED BY coin_ledger.id;

CREATE TABLE crom_pages (
    url text NOT NULL,
    site_url text NOT NULL,
    category text DEFAULT '_default'::text NOT NULL,
    created_at timestamp with time zone,
    rev_count integer DEFAULT 0 NOT NULL,
    vote_count integer DEFAULT 0 NOT NULL,
    rating integer DEFAULT 0 NOT NULL,
    comments integer DEFAULT 0 NOT NULL,
    is_private boolean DEFAULT false NOT NULL,
    fp text DEFAULT ''::text NOT NULL,
    detail_fp text,
    detail_at timestamp with time zone,
    seen_at timestamp with time zone DEFAULT now() NOT NULL,
    title text,
    alt_title text,
    title_at timestamp with time zone
);

CREATE VIEW crom_content_pages AS
 SELECT url,
    site_url,
    category,
    created_at,
    rev_count,
    vote_count,
    rating,
    comments,
    is_private,
    fp,
    detail_fp,
    detail_at,
    seen_at
   FROM crom_pages
  WHERE ((NOT is_private) AND (category <> ALL (ARRAY['fragment'::text, 'component'::text, 'theme'::text, 'admin'::text, 'system'::text, 'nav'::text, 'forum'::text, 'workbench'::text, 'sandbox'::text, 'draft'::text, '_template'::text, 'more-by'::text, 'review'::text, 'criticism'::text])));

CREATE TABLE crom_day_page (
    url text NOT NULL,
    site_url text NOT NULL,
    day date NOT NULL,
    edits integer DEFAULT 0 NOT NULL,
    rating_delta integer DEFAULT 0 NOT NULL
);

CREATE TABLE crom_day_voter (
    site_url text NOT NULL,
    day date NOT NULL,
    uid bigint NOT NULL
);

CREATE TABLE crom_sites (
    site_url text NOT NULL,
    display text DEFAULT ''::text NOT NULL,
    language text DEFAULT ''::text NOT NULL,
    page_count integer DEFAULT 0 NOT NULL,
    last_page_at timestamp with time zone,
    last_vote_at timestamp with time zone,
    last_rev_at timestamp with time zone,
    swept_at timestamp with time zone,
    detailed_at timestamp with time zone,
    sweep_cursor text,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);

CREATE TABLE forum_posts (
    id bigint NOT NULL,
    thread_id bigint NOT NULL,
    parent_id bigint,
    title text DEFAULT ''::text NOT NULL,
    author_uid text,
    created_at timestamp with time zone,
    edited_at timestamp with time zone,
    html text DEFAULT ''::text NOT NULL,
    content text DEFAULT ''::text NOT NULL,
    depth integer DEFAULT 0 NOT NULL,
    sort_index integer DEFAULT 0 NOT NULL,
    synced_at timestamp with time zone DEFAULT now() NOT NULL
);

CREATE TABLE gacha_cards (
    user_id bigint NOT NULL,
    article_id bigint NOT NULL,
    qty integer DEFAULT 0 NOT NULL,
    first_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT gacha_cards_qty_check CHECK ((qty >= 0))
);

CREATE TABLE gacha_listings (
    id bigint NOT NULL,
    seller_id bigint NOT NULL,
    article_id bigint NOT NULL,
    price bigint NOT NULL,
    status text DEFAULT 'open'::text NOT NULL,
    buyer_id bigint,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    closed_at timestamp with time zone,
    CONSTRAINT gacha_listings_price_check CHECK ((price > 0))
);

CREATE SEQUENCE gacha_listings_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE gacha_listings_id_seq OWNED BY gacha_listings.id;

CREATE VIEW gacha_pool AS
 SELECT id AS article_id,
    wiki,
    page,
    page_id,
    title,
    author,
    rating,
    tags,
    created_at,
    card_rarity(rating) AS rarity
   FROM articles a
  WHERE ((status <> 'deleted'::text) AND (author IS NOT NULL) AND (author <> ''::text) AND (page !~~ 'component:%'::text) AND (page !~~ 'theme:%'::text) AND (page !~~ 'system:%'::text) AND (page !~~ 'fragment:%'::text) AND (page !~~ 'admin:%'::text) AND (page !~~ 'help:%'::text) AND (page !~~ 'deleted:%'::text) AND (page !~~ 'css:%'::text) AND (page !~~ 'nav:%'::text) AND (page !~~ 'search:%'::text));

CREATE TABLE gacha_pulls (
    id bigint NOT NULL,
    user_id bigint NOT NULL,
    article_id bigint NOT NULL,
    rarity text NOT NULL,
    by_pity boolean DEFAULT false NOT NULL,
    batch text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);

CREATE SEQUENCE gacha_pulls_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE gacha_pulls_id_seq OWNED BY gacha_pulls.id;

CREATE TABLE gacha_state (
    user_id bigint NOT NULL,
    total_pulls bigint DEFAULT 0 NOT NULL,
    since_ssr integer DEFAULT 0 NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);

CREATE TABLE vote_events (
    id bigint NOT NULL,
    article_id bigint NOT NULL,
    uid text NOT NULL,
    username text,
    old_vote integer DEFAULT 0 NOT NULL,
    new_vote integer DEFAULT 0 NOT NULL,
    event_type text,
    voted_at timestamp with time zone NOT NULL
);

CREATE VIEW real_vote_events AS
 SELECT id,
    article_id,
    uid,
    username,
    old_vote,
    new_vote,
    event_type,
    voted_at
   FROM vote_events e
  ;

CREATE TABLE revisions (
    article_id bigint NOT NULL,
    rev_no integer NOT NULL,
    rev_id bigint,
    flags text,
    uid text,
    username text,
    comment text,
    changed_at timestamp with time zone
);

CREATE TABLE stock_auctions (
    id bigint NOT NULL,
    symbol text NOT NULL,
    at timestamp with time zone NOT NULL,
    price_before numeric(14,4) NOT NULL,
    price_after numeric(14,4) NOT NULL,
    fair numeric(14,4) NOT NULL,
    orders integer DEFAULT 0 NOT NULL,
    buy_qty bigint DEFAULT 0 NOT NULL,
    sell_qty bigint DEFAULT 0 NOT NULL,
    impact_pct double precision DEFAULT 0 NOT NULL,
    drift_pct double precision DEFAULT 0 NOT NULL,
    shock_pct double precision DEFAULT 0 NOT NULL,
    event_pct double precision DEFAULT 0 NOT NULL,
    new_pages integer DEFAULT 0 NOT NULL,
    expected double precision DEFAULT 0 NOT NULL,
    events jsonb DEFAULT '[]'::jsonb NOT NULL
);

CREATE SEQUENCE stock_auctions_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE stock_auctions_id_seq OWNED BY stock_auctions.id;

CREATE TABLE stock_bot_log (
    id bigint NOT NULL,
    at timestamp with time zone DEFAULT now() NOT NULL,
    bot text NOT NULL,
    action text NOT NULL,
    side text,
    qty bigint DEFAULT 0 NOT NULL,
    price numeric(12,4),
    note text DEFAULT ''::text NOT NULL,
    symbol text DEFAULT 'CLOUD'::text NOT NULL
);

CREATE SEQUENCE stock_bot_log_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE stock_bot_log_id_seq OWNED BY stock_bot_log.id;

CREATE TABLE stock_bots (
    name text NOT NULL,
    inventory bigint DEFAULT 0 NOT NULL,
    cash bigint DEFAULT 0 NOT NULL,
    avg_cost numeric(14,4) DEFAULT 0 NOT NULL,
    realized_pnl bigint DEFAULT 0 NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    symbol text DEFAULT 'CLOUD'::text NOT NULL
);

CREATE TABLE stock_liquidations (
    id bigint NOT NULL,
    user_id bigint NOT NULL,
    at timestamp with time zone DEFAULT now() NOT NULL,
    qty bigint NOT NULL,
    price numeric(14,4) NOT NULL,
    margin bigint NOT NULL,
    equity bigint NOT NULL,
    payout bigint NOT NULL,
    shortfall bigint DEFAULT 0 NOT NULL,
    symbol text DEFAULT 'CLOUD'::text NOT NULL
);

CREATE SEQUENCE stock_liquidations_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE stock_liquidations_id_seq OWNED BY stock_liquidations.id;

CREATE TABLE stock_maker (
    inventory bigint NOT NULL,
    cash bigint NOT NULL,
    p_ref numeric(14,4) NOT NULL,
    p_fair numeric(14,4) NOT NULL,
    reserve bigint DEFAULT 0 NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    symbol text NOT NULL
);

CREATE TABLE stock_orders (
    id bigint NOT NULL,
    user_id bigint,
    bot text,
    side text NOT NULL,
    kind text NOT NULL,
    price numeric(14,4),
    qty bigint NOT NULL,
    filled bigint DEFAULT 0 NOT NULL,
    hold_coins bigint DEFAULT 0 NOT NULL,
    status text DEFAULT 'open'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    closed_at timestamp with time zone,
    leverage numeric(6,2) DEFAULT 1 NOT NULL,
    symbol text DEFAULT 'CLOUD'::text NOT NULL,
    CONSTRAINT stock_orders_filled_check CHECK ((filled >= 0)),
    CONSTRAINT stock_orders_kind_check CHECK ((kind = ANY (ARRAY['limit'::text, 'market'::text, 'auction'::text]))),
    CONSTRAINT stock_orders_qty_check CHECK ((qty > 0)),
    CONSTRAINT stock_orders_side_check CHECK ((side = ANY (ARRAY['buy'::text, 'sell'::text]))),
    CONSTRAINT stock_orders_status_check CHECK ((status = ANY (ARRAY['open'::text, 'filled'::text, 'cancelled'::text])))
);

CREATE SEQUENCE stock_orders_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE stock_orders_id_seq OWNED BY stock_orders.id;

CREATE TABLE stock_positions (
    user_id bigint NOT NULL,
    qty bigint DEFAULT 0 NOT NULL,
    avg_cost numeric(14,4) DEFAULT 0 NOT NULL,
    realized_pnl bigint DEFAULT 0 NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    locked_qty bigint DEFAULT 0 NOT NULL,
    margin bigint DEFAULT 0 NOT NULL,
    symbol text DEFAULT 'CLOUD'::text NOT NULL,
    CONSTRAINT stock_positions_locked_qty_check CHECK ((locked_qty >= 0))
);

CREATE TABLE stock_symbols (
    code text NOT NULL,
    name text NOT NULL,
    short_name text DEFAULT ''::text NOT NULL,
    source text DEFAULT 'local'::text NOT NULL,
    site_url text,
    enabled boolean DEFAULT false NOT NULL,
    listed_at timestamp with time zone,
    sort integer DEFAULT 0 NOT NULL,
    params jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT stock_symbols_site_url_required CHECK (((source = 'local'::text) OR (site_url IS NOT NULL))),
    CONSTRAINT stock_symbols_source_check CHECK ((source = ANY (ARRAY['local'::text, 'crom'::text])))
);

CREATE TABLE stock_ticks (
    at timestamp with time zone NOT NULL,
    open numeric(14,4) NOT NULL,
    high numeric(14,4) NOT NULL,
    low numeric(14,4) NOT NULL,
    close numeric(14,4) NOT NULL,
    volume bigint DEFAULT 0 NOT NULL,
    p_fair numeric(14,4) NOT NULL,
    net_qty bigint DEFAULT 0 NOT NULL,
    symbol text DEFAULT 'CLOUD'::text NOT NULL
);

CREATE TABLE stock_trades (
    id bigint NOT NULL,
    user_id bigint,
    side text NOT NULL,
    qty bigint NOT NULL,
    price numeric(14,4) NOT NULL,
    gross bigint NOT NULL,
    fee bigint NOT NULL,
    at timestamp with time zone DEFAULT now() NOT NULL,
    taker_side text,
    bot text,
    buy_order_id bigint,
    sell_order_id bigint,
    symbol text DEFAULT 'CLOUD'::text NOT NULL,
    CONSTRAINT stock_trades_qty_check CHECK ((qty > 0))
);

CREATE SEQUENCE stock_trades_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE stock_trades_id_seq OWNED BY stock_trades.id;

CREATE TABLE sync_state (
    key text NOT NULL,
    value jsonb DEFAULT '{}'::jsonb NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);

CREATE TABLE task_defs (
    code text NOT NULL,
    kind text NOT NULL,
    title text NOT NULL,
    description text,
    reward bigint NOT NULL,
    target integer DEFAULT 1 NOT NULL,
    sort integer DEFAULT 0 NOT NULL,
    enabled boolean DEFAULT true NOT NULL
);

CREATE TABLE task_progress (
    user_id bigint NOT NULL,
    task_code text NOT NULL,
    period text NOT NULL,
    progress integer DEFAULT 0 NOT NULL,
    target integer NOT NULL,
    completed_at timestamp with time zone,
    claimed_at timestamp with time zone,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);

CREATE TABLE users (
    id bigint NOT NULL,
    pwikit_id bigint NOT NULL,
    name text NOT NULL,
    avatar_url text,
    role text DEFAULT 'user'::text NOT NULL,
    coins bigint DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    last_active_at timestamp with time zone,
    banned_at timestamp with time zone,
    CONSTRAINT users_coins_check CHECK ((coins >= 0))
);

CREATE SEQUENCE users_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE users_id_seq OWNED BY users.id;

CREATE SEQUENCE vote_events_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE vote_events_id_seq OWNED BY vote_events.id;

ALTER TABLE ONLY articles ALTER COLUMN id SET DEFAULT nextval('articles_id_seq'::regclass);

ALTER TABLE ONLY coin_ledger ALTER COLUMN id SET DEFAULT nextval('coin_ledger_id_seq'::regclass);

ALTER TABLE ONLY gacha_listings ALTER COLUMN id SET DEFAULT nextval('gacha_listings_id_seq'::regclass);

ALTER TABLE ONLY gacha_pulls ALTER COLUMN id SET DEFAULT nextval('gacha_pulls_id_seq'::regclass);

ALTER TABLE ONLY stock_auctions ALTER COLUMN id SET DEFAULT nextval('stock_auctions_id_seq'::regclass);

ALTER TABLE ONLY stock_bot_log ALTER COLUMN id SET DEFAULT nextval('stock_bot_log_id_seq'::regclass);

ALTER TABLE ONLY stock_liquidations ALTER COLUMN id SET DEFAULT nextval('stock_liquidations_id_seq'::regclass);

ALTER TABLE ONLY stock_orders ALTER COLUMN id SET DEFAULT nextval('stock_orders_id_seq'::regclass);

ALTER TABLE ONLY stock_trades ALTER COLUMN id SET DEFAULT nextval('stock_trades_id_seq'::regclass);

ALTER TABLE ONLY users ALTER COLUMN id SET DEFAULT nextval('users_id_seq'::regclass);

ALTER TABLE ONLY vote_events ALTER COLUMN id SET DEFAULT nextval('vote_events_id_seq'::regclass);

ALTER TABLE ONLY articles
    ADD CONSTRAINT articles_pkey PRIMARY KEY (id);

ALTER TABLE ONLY coin_ledger
    ADD CONSTRAINT coin_ledger_pkey PRIMARY KEY (id);

ALTER TABLE ONLY crom_day_page
    ADD CONSTRAINT crom_day_page_pkey PRIMARY KEY (url, day);

ALTER TABLE ONLY crom_day_voter
    ADD CONSTRAINT crom_day_voter_pkey PRIMARY KEY (site_url, day, uid);

ALTER TABLE ONLY crom_pages
    ADD CONSTRAINT crom_pages_pkey PRIMARY KEY (url);

ALTER TABLE ONLY crom_sites
    ADD CONSTRAINT crom_sites_pkey PRIMARY KEY (site_url);

ALTER TABLE ONLY forum_posts
    ADD CONSTRAINT forum_posts_pkey PRIMARY KEY (id);

ALTER TABLE ONLY gacha_cards
    ADD CONSTRAINT gacha_cards_pkey PRIMARY KEY (user_id, article_id);

ALTER TABLE ONLY gacha_listings
    ADD CONSTRAINT gacha_listings_pkey PRIMARY KEY (id);

ALTER TABLE ONLY gacha_pulls
    ADD CONSTRAINT gacha_pulls_pkey PRIMARY KEY (id);

ALTER TABLE ONLY gacha_state
    ADD CONSTRAINT gacha_state_pkey PRIMARY KEY (user_id);

ALTER TABLE ONLY revisions
    ADD CONSTRAINT revisions_pkey PRIMARY KEY (article_id, rev_no);

ALTER TABLE ONLY stock_auctions
    ADD CONSTRAINT stock_auctions_pkey PRIMARY KEY (id);

ALTER TABLE ONLY stock_auctions
    ADD CONSTRAINT stock_auctions_symbol_at_key UNIQUE (symbol, at);

ALTER TABLE ONLY stock_bot_log
    ADD CONSTRAINT stock_bot_log_pkey PRIMARY KEY (id);

ALTER TABLE ONLY stock_bots
    ADD CONSTRAINT stock_bots_pkey PRIMARY KEY (symbol, name);

ALTER TABLE ONLY stock_liquidations
    ADD CONSTRAINT stock_liquidations_pkey PRIMARY KEY (id);

ALTER TABLE ONLY stock_maker
    ADD CONSTRAINT stock_maker_pkey PRIMARY KEY (symbol);

ALTER TABLE ONLY stock_orders
    ADD CONSTRAINT stock_orders_pkey PRIMARY KEY (id);

ALTER TABLE ONLY stock_positions
    ADD CONSTRAINT stock_positions_pkey PRIMARY KEY (user_id, symbol);

ALTER TABLE ONLY stock_symbols
    ADD CONSTRAINT stock_symbols_pkey PRIMARY KEY (code);

ALTER TABLE ONLY stock_ticks
    ADD CONSTRAINT stock_ticks_pkey PRIMARY KEY (symbol, at);

ALTER TABLE ONLY stock_trades
    ADD CONSTRAINT stock_trades_pkey PRIMARY KEY (id);

ALTER TABLE ONLY sync_state
    ADD CONSTRAINT sync_state_pkey PRIMARY KEY (key);

ALTER TABLE ONLY task_defs
    ADD CONSTRAINT task_defs_pkey PRIMARY KEY (code);

ALTER TABLE ONLY task_progress
    ADD CONSTRAINT task_progress_pkey PRIMARY KEY (user_id, task_code, period);

ALTER TABLE ONLY users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);

ALTER TABLE ONLY vote_events
    ADD CONSTRAINT vote_events_article_id_uid_voted_at_new_vote_key UNIQUE (article_id, uid, voted_at, new_vote);

ALTER TABLE ONLY vote_events
    ADD CONSTRAINT vote_events_pkey PRIMARY KEY (id);

CREATE INDEX articles_author_idx ON articles USING btree (lower(author));

CREATE INDEX articles_content_trgm_idx ON articles USING gin (content gin_trgm_ops);

CREATE INDEX articles_created_idx ON articles USING btree (created_at DESC);

CREATE INDEX articles_lastmod_idx ON articles USING btree (lastmod DESC);

CREATE INDEX articles_rating_idx ON articles USING btree (rating DESC);

CREATE INDEX articles_revs_todo_idx ON articles USING btree (lastmod DESC NULLS LAST) WHERE ((status <> 'deleted'::text) AND (revs_synced_mark IS DISTINCT FROM revision_count));

CREATE INDEX articles_source_todo_idx ON articles USING btree (source_fetched_at NULLS FIRST, lastmod DESC) WHERE (status <> 'deleted'::text);

CREATE INDEX articles_status_idx ON articles USING btree (status);

CREATE INDEX articles_tags_idx ON articles USING gin (tags);

CREATE INDEX articles_title_trgm_idx ON articles USING gin (title gin_trgm_ops);

CREATE INDEX articles_votes_todo_idx ON articles USING btree (lastmod DESC NULLS LAST) WHERE ((status <> 'deleted'::text) AND (votes_synced_mark IS DISTINCT FROM rating_votes));

CREATE INDEX articles_wiki_idx ON articles USING btree (wiki);

CREATE INDEX articles_wiki_page_idx ON articles USING btree (wiki, page);

CREATE UNIQUE INDEX articles_wiki_page_live_uidx ON articles USING btree (wiki, page) WHERE (status <> 'deleted'::text);

CREATE UNIQUE INDEX articles_wiki_pageid_uidx ON articles USING btree (wiki, page_id) WHERE (page_id IS NOT NULL);

CREATE INDEX coin_ledger_user_idx ON coin_ledger USING btree (user_id, created_at DESC);

CREATE INDEX crom_day_page_site_day_idx ON crom_day_page USING btree (site_url, day);

CREATE INDEX crom_pages_dirty_idx ON crom_pages USING btree (site_url) WHERE ((detail_fp IS NULL) OR (detail_fp <> fp));

CREATE INDEX crom_pages_site_created_idx ON crom_pages USING btree (site_url, created_at);

CREATE INDEX crom_pages_title_todo_idx ON crom_pages USING btree (created_at DESC) WHERE (title_at IS NULL);

CREATE INDEX forum_posts_content_idx ON forum_posts USING gin (content gin_trgm_ops);

CREATE INDEX forum_posts_thread_idx ON forum_posts USING btree (thread_id, sort_index);

CREATE INDEX forum_posts_uid_idx ON forum_posts USING btree (author_uid, created_at DESC);

CREATE INDEX gacha_cards_article_idx ON gacha_cards USING btree (article_id);

CREATE INDEX gacha_cards_user_idx ON gacha_cards USING btree (user_id) WHERE (qty > 0);

CREATE INDEX gacha_listings_open_idx ON gacha_listings USING btree (article_id, price) WHERE (status = 'open'::text);

CREATE INDEX gacha_listings_seller_idx ON gacha_listings USING btree (seller_id, created_at DESC);

CREATE INDEX gacha_pulls_user_idx ON gacha_pulls USING btree (user_id, created_at DESC);

CREATE INDEX revisions_time_idx ON revisions USING btree (changed_at DESC);

CREATE INDEX revisions_uid_time_idx ON revisions USING btree (uid, changed_at DESC);

CREATE INDEX stock_auctions_symbol_at_idx ON stock_auctions USING btree (symbol, at DESC);

CREATE INDEX stock_bot_log_symbol_at_idx ON stock_bot_log USING btree (symbol, at DESC);

CREATE INDEX stock_liquidations_user_idx ON stock_liquidations USING btree (user_id, at DESC);

CREATE INDEX stock_orders_asks_idx ON stock_orders USING btree (symbol, price, id) WHERE ((status = 'open'::text) AND (side = 'sell'::text));

CREATE INDEX stock_orders_bids_idx ON stock_orders USING btree (symbol, price DESC, id) WHERE ((status = 'open'::text) AND (side = 'buy'::text));

CREATE INDEX stock_orders_bot_idx ON stock_orders USING btree (symbol, bot) WHERE ((status = 'open'::text) AND (bot IS NOT NULL));

CREATE UNIQUE INDEX stock_orders_one_pending_idx ON stock_orders USING btree (user_id, symbol) WHERE ((status = 'open'::text) AND (user_id IS NOT NULL));

CREATE INDEX stock_orders_user_idx ON stock_orders USING btree (user_id, id DESC);

CREATE INDEX stock_orders_user_open_idx ON stock_orders USING btree (user_id, symbol) WHERE (status = 'open'::text);

CREATE INDEX stock_positions_open_idx ON stock_positions USING btree (symbol, user_id) WHERE (qty <> 0);

CREATE INDEX stock_symbols_enabled_idx ON stock_symbols USING btree (sort, code) WHERE enabled;

CREATE INDEX stock_ticks_symbol_at_idx ON stock_ticks USING btree (symbol, at DESC);

CREATE INDEX stock_trades_symbol_at_idx ON stock_trades USING btree (symbol, at DESC);

CREATE INDEX stock_trades_user_idx ON stock_trades USING btree (user_id, id DESC);

CREATE INDEX stock_trades_user_symbol_idx ON stock_trades USING btree (user_id, symbol, id DESC);

CREATE INDEX task_progress_user_period_idx ON task_progress USING btree (user_id, period);

CREATE INDEX users_active_idx ON users USING btree (last_active_at DESC);

CREATE INDEX users_coins_idx ON users USING btree (coins DESC);


CREATE UNIQUE INDEX users_pwikit_uidx ON users USING btree (pwikit_id);

CREATE INDEX vote_events_article_idx ON vote_events USING btree (article_id, voted_at);

CREATE INDEX vote_events_time_idx ON vote_events USING btree (voted_at DESC);

CREATE INDEX vote_events_uid_time_idx ON vote_events USING btree (uid, voted_at DESC);

CREATE TRIGGER stock_positions_return_shares BEFORE DELETE ON stock_positions FOR EACH ROW EXECUTE FUNCTION stock_return_shares_on_delete();

ALTER TABLE ONLY coin_ledger
    ADD CONSTRAINT coin_ledger_operator_id_fkey FOREIGN KEY (operator_id) REFERENCES users(id);

ALTER TABLE ONLY coin_ledger
    ADD CONSTRAINT coin_ledger_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

ALTER TABLE ONLY crom_day_page
    ADD CONSTRAINT crom_day_page_url_fkey FOREIGN KEY (url) REFERENCES crom_pages(url) ON DELETE CASCADE;

ALTER TABLE ONLY crom_pages
    ADD CONSTRAINT crom_pages_site_url_fkey FOREIGN KEY (site_url) REFERENCES crom_sites(site_url) ON DELETE CASCADE;

ALTER TABLE ONLY gacha_cards
    ADD CONSTRAINT gacha_cards_article_id_fkey FOREIGN KEY (article_id) REFERENCES articles(id) ON DELETE CASCADE;

ALTER TABLE ONLY gacha_cards
    ADD CONSTRAINT gacha_cards_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

ALTER TABLE ONLY gacha_listings
    ADD CONSTRAINT gacha_listings_article_id_fkey FOREIGN KEY (article_id) REFERENCES articles(id) ON DELETE CASCADE;

ALTER TABLE ONLY gacha_listings
    ADD CONSTRAINT gacha_listings_buyer_id_fkey FOREIGN KEY (buyer_id) REFERENCES users(id);

ALTER TABLE ONLY gacha_listings
    ADD CONSTRAINT gacha_listings_seller_id_fkey FOREIGN KEY (seller_id) REFERENCES users(id) ON DELETE CASCADE;

ALTER TABLE ONLY gacha_pulls
    ADD CONSTRAINT gacha_pulls_article_id_fkey FOREIGN KEY (article_id) REFERENCES articles(id) ON DELETE CASCADE;

ALTER TABLE ONLY gacha_pulls
    ADD CONSTRAINT gacha_pulls_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

ALTER TABLE ONLY gacha_state
    ADD CONSTRAINT gacha_state_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

ALTER TABLE ONLY revisions
    ADD CONSTRAINT revisions_article_id_fkey FOREIGN KEY (article_id) REFERENCES articles(id) ON DELETE CASCADE;

ALTER TABLE ONLY stock_auctions
    ADD CONSTRAINT stock_auctions_symbol_fkey FOREIGN KEY (symbol) REFERENCES stock_symbols(code) ON UPDATE CASCADE;

ALTER TABLE ONLY stock_bots
    ADD CONSTRAINT stock_bots_symbol_fkey FOREIGN KEY (symbol) REFERENCES stock_symbols(code) ON UPDATE CASCADE;

ALTER TABLE ONLY stock_liquidations
    ADD CONSTRAINT stock_liquidations_symbol_fkey FOREIGN KEY (symbol) REFERENCES stock_symbols(code) ON UPDATE CASCADE;

ALTER TABLE ONLY stock_liquidations
    ADD CONSTRAINT stock_liquidations_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

ALTER TABLE ONLY stock_maker
    ADD CONSTRAINT stock_maker_symbol_fkey FOREIGN KEY (symbol) REFERENCES stock_symbols(code) ON UPDATE CASCADE;

ALTER TABLE ONLY stock_orders
    ADD CONSTRAINT stock_orders_symbol_fkey FOREIGN KEY (symbol) REFERENCES stock_symbols(code) ON UPDATE CASCADE;

ALTER TABLE ONLY stock_orders
    ADD CONSTRAINT stock_orders_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

ALTER TABLE ONLY stock_positions
    ADD CONSTRAINT stock_positions_symbol_fkey FOREIGN KEY (symbol) REFERENCES stock_symbols(code) ON UPDATE CASCADE;

ALTER TABLE ONLY stock_positions
    ADD CONSTRAINT stock_positions_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

ALTER TABLE ONLY stock_ticks
    ADD CONSTRAINT stock_ticks_symbol_fkey FOREIGN KEY (symbol) REFERENCES stock_symbols(code) ON UPDATE CASCADE;

ALTER TABLE ONLY stock_trades
    ADD CONSTRAINT stock_trades_symbol_fkey FOREIGN KEY (symbol) REFERENCES stock_symbols(code) ON UPDATE CASCADE;

ALTER TABLE ONLY stock_trades
    ADD CONSTRAINT stock_trades_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

ALTER TABLE ONLY task_progress
    ADD CONSTRAINT task_progress_task_code_fkey FOREIGN KEY (task_code) REFERENCES task_defs(code) ON DELETE CASCADE;

ALTER TABLE ONLY task_progress
    ADD CONSTRAINT task_progress_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

ALTER TABLE ONLY vote_events
    ADD CONSTRAINT vote_events_article_id_fkey FOREIGN KEY (article_id) REFERENCES articles(id) ON DELETE CASCADE;
