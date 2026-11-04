package httpx

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
)

type APIError struct {
	Status  int    `json:"-"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *APIError) Error() string { return e.Message }

func Err(status int, code, format string, args ...any) *APIError {
	msg := format
	if len(args) > 0 {
		msg = fmt.Sprintf(format, args...)
	}
	return &APIError{Status: status, Code: code, Message: msg}
}

func BadRequestMsg(msg string) *APIError {
	return &APIError{Status: http.StatusBadRequest, Code: "bad_request", Message: msg}
}

func BadRequest(format string, args ...any) *APIError {
	return Err(http.StatusBadRequest, "bad_request", format, args...)
}

func Unauthorized(format string, args ...any) *APIError {
	return Err(http.StatusUnauthorized, "unauthorized", format, args...)
}

func Forbidden(format string, args ...any) *APIError {
	return Err(http.StatusForbidden, "forbidden", format, args...)
}

func NotFound(format string, args ...any) *APIError {
	return Err(http.StatusNotFound, "not_found", format, args...)
}

func Conflict(format string, args ...any) *APIError {
	return Err(http.StatusConflict, "conflict", format, args...)
}

func TooMany(format string, args ...any) *APIError {
	return Err(http.StatusTooManyRequests, "rate_limited", format, args...)
}

type Handler func(w http.ResponseWriter, r *http.Request) error

func Wrap(h Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := h(w, r); err != nil {
			WriteError(w, r, err)
		}
	}
}

func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		JSON(w, apiErr.Status, map[string]any{"error": apiErr})
		return
	}
	slog.Error("request failed", "path", r.URL.Path, "method", r.Method, "err", err)
	JSON(w, http.StatusInternalServerError, map[string]any{
		"error": &APIError{Code: "internal", Message: "服务器内部错误"},
	})
}

func JSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if body == nil {
		return
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(body); err != nil {
		slog.Error("write json response", "err", err)
	}
}

func OK(w http.ResponseWriter, body any) {
	JSON(w, http.StatusOK, body)
}

func DecodeJSON(r *http.Request, dst any) error {
	if r.Body == nil {
		return BadRequest("请求体为空")
	}
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return BadRequest("请求体格式错误：%v", err)
	}
	return nil
}

func QueryString(r *http.Request, key, def string) string {
	if v := strings.TrimSpace(r.URL.Query().Get(key)); v != "" {
		return v
	}
	return def
}

func QueryInt(r *http.Request, key string, def, min, max int) int {
	v := strings.TrimSpace(r.URL.Query().Get(key))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	if n < min {
		return min
	}
	if n > max {
		return max
	}
	return n
}

func QueryBool(r *http.Request, key string, def bool) bool {
	v := strings.TrimSpace(r.URL.Query().Get(key))
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}

func QueryInt64(r *http.Request, key string, def int64) int64 {
	v := strings.TrimSpace(r.URL.Query().Get(key))
	if v == "" {
		return def
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return def
	}
	return n
}

type Page struct {
	Page   int
	Size   int
	Offset int
}

func ParsePage(r *http.Request, defSize, maxSize int) Page {
	p := QueryInt(r, "page", 1, 1, 100000)
	size := QueryInt(r, "limit", defSize, 1, maxSize)
	return Page{Page: p, Size: size, Offset: (p - 1) * size}
}

type PageMeta struct {
	Total    int  `json:"total"`
	Page     int  `json:"page"`
	PageSize int  `json:"pageSize"`
	HasNext  bool `json:"hasNext"`
}

func Meta(p Page, total int) PageMeta {
	return PageMeta{
		Total:    total,
		Page:     p.Page,
		PageSize: p.Size,
		HasNext:  p.Offset+p.Size < total,
	}
}
