package api

import (
	"errors"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"net/http"

	"github.com/scpwikicloud/scpper/internal/httpx"
)

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func urlParam(r *http.Request, key string) string {
	return strings.TrimSpace(chi.URLParam(r, key))
}

func urlParamInt64(r *http.Request, key string) (int64, error) {
	raw := urlParam(r, key)
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n <= 0 {
		return 0, httpx.BadRequest("参数 %s 无效", key)
	}
	return n, nil
}

type argBuilder struct {
	args []any
}

func (b *argBuilder) add(v any) string {
	b.args = append(b.args, v)
	return "$" + strconv.Itoa(len(b.args))
}

func (b *argBuilder) all() []any { return b.args }

func escapeLike(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s)
}

func likeContains(s string) string {
	return "%" + escapeLike(s) + "%"
}

func splitWords(s string) []string {
	f := strings.FieldsFunc(s, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '　' || r == '\v' || r == '\f'
	})
	out := make([]string, 0, len(f))
	for _, w := range f {
		if w = strings.TrimSpace(w); w != "" {
			out = append(out, w)
		}
	}
	return out
}

func splitTags(s string) (include, exclude []string) {
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == '\r' ||
			r == '　' || r == ',' || r == '，' || r == '、'
	})
	for _, t := range fields {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		if strings.HasPrefix(t, "-") {
			if ex := strings.TrimSpace(t[1:]); ex != "" {
				exclude = append(exclude, ex)
			}
			continue
		}
		include = append(include, t)
	}
	return include, exclude
}

func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

func itoa(n int64) string {
	return strconv.FormatInt(n, 10)
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
