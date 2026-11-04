package crom

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

const (
	maxRetries = 5
	baseDelay  = 800 * time.Millisecond

	SweepPageSize = 100

	DetailBatchSize = 40

	reservePoints = 3000
)

type Client struct {
	endpoint string
	http     *http.Client
	limiter  *rate.Limiter

	mu        sync.Mutex
	remaining int
	limit     int
	resetAt   time.Time
	known     bool
}

func New(endpoint string, qps float64, burst int) *Client {
	if endpoint == "" {
		endpoint = "https://apiv1.crom.avn.sh/graphql"
	}
	return &Client{
		endpoint: endpoint,
		http: &http.Client{
			Timeout: 90 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        20,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		limiter: rate.NewLimiter(rate.Limit(qps), burst),
	}
}

func (c *Client) Budget() (remaining, limit int, resetAt time.Time, known bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.remaining, c.limit, c.resetAt, c.known
}

type Page struct {
	URL       string
	Category  string
	CreatedAt time.Time
	RevCount  int
	VoteCount int
	Rating    int
	Comments  int
	IsPrivate bool
}

func (p Page) Fingerprint() string {
	return strconv.Itoa(p.RevCount) + ":" + strconv.Itoa(p.VoteCount) + ":" +
		strconv.Itoa(p.Rating) + ":" + strconv.Itoa(p.Comments)
}

type Revision struct {
	At  time.Time
	UID int64
}

type Vote struct {
	Day       time.Time
	UID       int64
	Direction int
}

type Detail struct {
	Page
	Found     bool
	Revisions []Revision
	Votes     []Vote
}

type SiteInfo struct {
	Type        string
	DisplayName string
	URL         string
	Language    string
}

const wikidotLightFields = `category createdAt revisionCount voteCount rating commentCount isPrivate`

const wikidotHeavyFields = wikidotLightFields + `
	revisions { timestamp userWikidotId }
	coarseVoteRecords { timestamp userWikidotId direction }`

type wikidotInfoJSON struct {
	Category     string  `json:"category"`
	CreatedAt    string  `json:"createdAt"`
	RevisionCnt  int     `json:"revisionCount"`
	VoteCount    *int    `json:"voteCount"`
	Rating       *int    `json:"rating"`
	CommentCount *int    `json:"commentCount"`
	IsPrivate    bool    `json:"isPrivate"`
	Revisions    []revJS `json:"revisions"`
	Votes        []votJS `json:"coarseVoteRecords"`
}

type revJS struct {
	Timestamp string `json:"timestamp"`
	UID       *int64 `json:"userWikidotId"`
}

type votJS struct {
	Timestamp string `json:"timestamp"`
	UID       int64  `json:"userWikidotId"`
	Direction int    `json:"direction"`
}

type pageJSON struct {
	URL         string           `json:"url"`
	WikidotInfo *wikidotInfoJSON `json:"wikidotInfo"`
}

func (w *wikidotInfoJSON) toPage(url string) Page {
	p := Page{
		URL:       url,
		Category:  w.Category,
		RevCount:  w.RevisionCnt,
		IsPrivate: w.IsPrivate,
	}
	if w.Category == "" {
		p.Category = "_default"
	}
	if w.VoteCount != nil {
		p.VoteCount = *w.VoteCount
	}
	if w.Rating != nil {
		p.Rating = *w.Rating
	}
	if w.CommentCount != nil {
		p.Comments = *w.CommentCount
	}
	p.CreatedAt, _ = parseTime(w.CreatedAt)
	return p
}

func (w *wikidotInfoJSON) toDetail(url string) Detail {
	d := Detail{Page: w.toPage(url), Found: true}
	for _, r := range w.Revisions {
		at, ok := parseTime(r.Timestamp)
		if !ok {
			continue
		}
		var uid int64
		if r.UID != nil {
			uid = *r.UID
		}
		d.Revisions = append(d.Revisions, Revision{At: at, UID: uid})
	}
	for _, v := range w.Votes {
		at, ok := parseTime(v.Timestamp)
		if !ok {
			continue
		}
		d.Votes = append(d.Votes, Vote{Day: at.UTC().Truncate(24 * time.Hour), UID: v.UID, Direction: v.Direction})
	}
	return d
}

func parseTime(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

func (c *Client) Sites(ctx context.Context) ([]SiteInfo, error) {
	var out struct {
		Sites []struct {
			Type        *string `json:"type"`
			DisplayName string  `json:"displayName"`
			URL         string  `json:"url"`
			Language    string  `json:"language"`
		} `json:"sites"`
	}
	if err := c.do(ctx, `{ sites { type displayName url language } rateLimit { cost remaining limit resetAt } }`, nil, &out); err != nil {
		return nil, err
	}
	list := make([]SiteInfo, 0, len(out.Sites))
	for _, s := range out.Sites {
		si := SiteInfo{DisplayName: s.DisplayName, URL: s.URL, Language: s.Language}
		if s.Type != nil {
			si.Type = *s.Type
		}
		list = append(list, si)
	}
	return list, nil
}

type SweepOrder int

const (
	ByURL SweepOrder = iota

	ByNewest
)

type SweepResult struct {
	Pages      []Page
	NextCursor string
	HasNext    bool
}

func (c *Client) Sweep(ctx context.Context, siteURL, cursor string, order SweepOrder, first int) (SweepResult, error) {
	if first <= 0 || first > SweepPageSize {
		first = SweepPageSize
	}
	sort := `{key: URL, order: ASC}`
	if order == ByNewest {
		sort = `{key: CREATED_AT, order: DESC}`
	}

	q := fmt.Sprintf(`query($u: String!, $after: ID, $first: Int!) {
		pages(filter: {url: {startsWith: $u}}, sort: %s, first: $first, after: $after) {
			pageInfo { hasNextPage endCursor }
			edges { node { url wikidotInfo { %s } } }
		}
		rateLimit { cost remaining limit resetAt }
	}`, sort, wikidotLightFields)

	vars := map[string]any{"u": siteURL, "first": first}
	if cursor != "" {
		vars["after"] = cursor
	}

	var out struct {
		Pages struct {
			PageInfo struct {
				HasNextPage bool   `json:"hasNextPage"`
				EndCursor   string `json:"endCursor"`
			} `json:"pageInfo"`
			Edges []struct {
				Node pageJSON `json:"node"`
			} `json:"edges"`
		} `json:"pages"`
	}
	if err := c.do(ctx, q, vars, &out); err != nil {
		return SweepResult{}, err
	}

	res := SweepResult{
		NextCursor: out.Pages.PageInfo.EndCursor,
		HasNext:    out.Pages.PageInfo.HasNextPage,
		Pages:      make([]Page, 0, len(out.Pages.Edges)),
	}
	for _, e := range out.Pages.Edges {

		if e.Node.WikidotInfo == nil {
			continue
		}
		res.Pages = append(res.Pages, e.Node.WikidotInfo.toPage(e.Node.URL))
	}
	return res, nil
}

func (c *Client) SweepHeavy(ctx context.Context, siteURL, cursor string, first int) ([]Detail, string, bool, error) {
	if first <= 0 || first > SweepPageSize {
		first = SweepPageSize
	}
	q := fmt.Sprintf(`query($u: String!, $after: ID, $first: Int!) {
		pages(filter: {url: {startsWith: $u}}, sort: {key: URL, order: ASC}, first: $first, after: $after) {
			pageInfo { hasNextPage endCursor }
			edges { node { url wikidotInfo { %s } } }
		}
		rateLimit { cost remaining limit resetAt }
	}`, wikidotHeavyFields)

	vars := map[string]any{"u": siteURL, "first": first}
	if cursor != "" {
		vars["after"] = cursor
	}

	var out struct {
		Pages struct {
			PageInfo struct {
				HasNextPage bool   `json:"hasNextPage"`
				EndCursor   string `json:"endCursor"`
			} `json:"pageInfo"`
			Edges []struct {
				Node pageJSON `json:"node"`
			} `json:"edges"`
		} `json:"pages"`
	}
	if err := c.do(ctx, q, vars, &out); err != nil {
		return nil, "", false, err
	}

	list := make([]Detail, 0, len(out.Pages.Edges))
	for _, e := range out.Pages.Edges {
		if e.Node.WikidotInfo == nil {
			continue
		}
		list = append(list, e.Node.WikidotInfo.toDetail(e.Node.URL))
	}
	return list, out.Pages.PageInfo.EndCursor, out.Pages.PageInfo.HasNextPage, nil
}

func (c *Client) Details(ctx context.Context, urls []string) ([]Detail, error) {
	if len(urls) == 0 {
		return nil, nil
	}
	if len(urls) > DetailBatchSize {
		urls = urls[:DetailBatchSize]
	}

	var b strings.Builder
	b.WriteString("query {\n")
	for i, u := range urls {
		raw, err := json.Marshal(u)
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(&b, "  p%d: page(url: %s) { url wikidotInfo { %s } }\n", i, raw, wikidotHeavyFields)
	}
	b.WriteString("  rateLimit { cost remaining limit resetAt }\n}")

	var out map[string]json.RawMessage
	if err := c.do(ctx, b.String(), nil, &out); err != nil {
		return nil, err
	}

	list := make([]Detail, 0, len(urls))
	for i, u := range urls {
		raw, ok := out["p"+strconv.Itoa(i)]
		if !ok || len(raw) == 0 || string(raw) == "null" {
			list = append(list, Detail{Page: Page{URL: u}})
			continue
		}
		var pj pageJSON
		if err := json.Unmarshal(raw, &pj); err != nil {
			return nil, fmt.Errorf("parse %s: %w", u, err)
		}
		if pj.WikidotInfo == nil {
			list = append(list, Detail{Page: Page{URL: u}})
			continue
		}

		list = append(list, pj.WikidotInfo.toDetail(u))
	}
	return list, nil
}

type Title struct {
	Found bool
	Title string
	Alt   string
}

func (c *Client) Titles(ctx context.Context, urls []string) (map[string]Title, error) {
	if len(urls) == 0 {
		return nil, nil
	}
	if len(urls) > DetailBatchSize {
		urls = urls[:DetailBatchSize]
	}

	var b strings.Builder
	b.WriteString("query {\n")
	for i, u := range urls {
		raw, err := json.Marshal(u)
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(&b, "  p%d: page(url: %s) { alternateTitles { type title } wikidotInfo { title } }\n", i, raw)
	}
	b.WriteString("  rateLimit { cost remaining limit resetAt }\n}")

	var out map[string]json.RawMessage
	if err := c.do(ctx, b.String(), nil, &out); err != nil {
		return nil, err
	}

	res := make(map[string]Title, len(urls))
	for i, u := range urls {
		raw, ok := out["p"+strconv.Itoa(i)]
		if !ok || len(raw) == 0 || string(raw) == "null" {
			res[u] = Title{}
			continue
		}
		var pj struct {
			Alt []struct {
				Type  string `json:"type"`
				Title string `json:"title"`
			} `json:"alternateTitles"`
			Info *struct {
				Title string `json:"title"`
			} `json:"wikidotInfo"`
		}
		if err := json.Unmarshal(raw, &pj); err != nil {
			return nil, fmt.Errorf("parse %s: %w", u, err)
		}
		t := Title{Found: pj.Info != nil}
		if pj.Info != nil {
			t.Title = strings.TrimSpace(pj.Info.Title)
		}
		site := u
		if i := strings.Index(u, ".wikidot.com"); i >= 0 {
			site = u[:i+len(".wikidot.com")]
		}
		for _, a := range pj.Alt {
			if strings.TrimSpace(a.Title) == "" {
				continue
			}
			if a.Type == site {
				t.Alt = strings.TrimSpace(a.Title)
				break
			}
			if t.Alt == "" {
				t.Alt = strings.TrimSpace(a.Title)
			}
		}
		res[u] = t
	}
	return res, nil
}

func (c *Client) CountPages(ctx context.Context, siteURL string) (int, error) {
	var out struct {
		Agg struct {
			Count int `json:"_count"`
		} `json:"agg"`
	}
	q := `query($u: String!) {
		agg: aggregatePageWikidotInfos(filter: {url: {startsWith: $u}}) { _count }
		rateLimit { cost remaining limit resetAt }
	}`
	if err := c.do(ctx, q, map[string]any{"u": siteURL}, &out); err != nil {
		return 0, err
	}
	return out.Agg.Count, nil
}

type gqlRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables,omitempty"`
}

type gqlError struct {
	Message string `json:"message"`
}

type gqlResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []gqlError      `json:"errors"`
}

type rateLimitJSON struct {
	Cost      int    `json:"cost"`
	Remaining int    `json:"remaining"`
	Limit     int    `json:"limit"`
	ResetAt   string `json:"resetAt"`
}

func (c *Client) do(ctx context.Context, query string, vars map[string]any, out any) error {
	body, err := json.Marshal(gqlRequest{Query: query, Variables: vars})
	if err != nil {
		return err
	}

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if err := c.waitForBudget(ctx); err != nil {
			return err
		}
		if err := c.limiter.Wait(ctx); err != nil {
			return err
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "scpper/1.0 (+https://github.com/scpwikicloud/scpper)")

		resp, err := c.http.Do(req)
		if err != nil {
			lastErr = err
			if !sleepCtx(ctx, backoff(attempt)) {
				return ctx.Err()
			}
			continue
		}

		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			delay := backoff(attempt)
			if ra := resp.Header.Get("Retry-After"); ra != "" {
				if secs, perr := time.ParseDuration(ra + "s"); perr == nil && secs > 0 {
					delay = secs
				}
			}
			resp.Body.Close()
			slog.Warn("crom throttled, retrying",
				"status", resp.StatusCode, "attempt", attempt, "delay", delay)
			lastErr = fmt.Errorf("crom HTTP %d", resp.StatusCode)
			if !sleepCtx(ctx, delay) {
				return ctx.Err()
			}
			continue
		}

		raw, err := io.ReadAll(io.LimitReader(resp.Body, 128<<20))
		resp.Body.Close()
		if err != nil {
			lastErr = err
			if !sleepCtx(ctx, backoff(attempt)) {
				return ctx.Err()
			}
			continue
		}

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("crom HTTP %d: %s", resp.StatusCode, truncate(string(raw), 300))
		}

		var gr gqlResponse
		if err := json.Unmarshal(raw, &gr); err != nil {
			return fmt.Errorf("parse crom response: %w (%s)", err, truncate(string(raw), 300))
		}
		c.noteBudget(gr.Data)
		if len(gr.Errors) > 0 {
			msgs := make([]string, 0, len(gr.Errors))
			for _, e := range gr.Errors {
				msgs = append(msgs, e.Message)
			}
			joined := strings.Join(msgs, "; ")
			// Crom answers a burst with 200 and this message rather than 429.
			if strings.Contains(strings.ToLower(joined), "too often") {
				delay := backoff(attempt)
				slog.Warn("crom throttled, retrying", "attempt", attempt, "delay", delay)
				lastErr = fmt.Errorf("crom graphql: %s", joined)
				if !sleepCtx(ctx, delay) {
					return ctx.Err()
				}
				continue
			}
			return fmt.Errorf("crom graphql: %s", joined)
		}
		if out == nil || len(gr.Data) == 0 {
			return nil
		}
		return json.Unmarshal(gr.Data, out)
	}
	return fmt.Errorf("crom retries exhausted: %w", lastErr)
}

func (c *Client) noteBudget(data json.RawMessage) {
	if len(data) == 0 {
		return
	}
	var probe struct {
		RateLimit *rateLimitJSON `json:"rateLimit"`
	}
	if err := json.Unmarshal(data, &probe); err != nil || probe.RateLimit == nil {
		return
	}
	reset, _ := parseTime(probe.RateLimit.ResetAt)

	c.mu.Lock()
	defer c.mu.Unlock()
	c.remaining = probe.RateLimit.Remaining
	c.limit = probe.RateLimit.Limit
	c.resetAt = reset
	c.known = true
}

func (c *Client) waitForBudget(ctx context.Context) error {
	c.mu.Lock()
	known, remaining, resetAt := c.known, c.remaining, c.resetAt
	c.mu.Unlock()

	if !known || remaining >= reservePoints || resetAt.IsZero() {
		return nil
	}
	wait := time.Until(resetAt) + 5*time.Second
	if wait <= 0 {
		return nil
	}
	if wait > time.Hour+time.Minute {
		wait = time.Hour + time.Minute
	}
	slog.Warn("crom quota low, waiting", "remaining", remaining, "wait", wait.Round(time.Second))
	if !sleepCtx(ctx, wait) {
		return ctx.Err()
	}
	c.mu.Lock()
	c.remaining = c.limit
	c.mu.Unlock()
	return nil
}

func backoff(attempt int) time.Duration {
	return time.Duration(float64(baseDelay) * math.Pow(2, float64(attempt)))
}

func sleepCtx(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
