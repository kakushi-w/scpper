package api

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

func (s *Server) notify(users []int64, title, message, link string) {
	if s.cfg.PwikitURL == "" || len(users) == 0 {
		return
	}
	body, err := json.Marshal(map[string]any{"users": users, "title": title, "message": message, "link": link})
	if err != nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.cfg.PwikitURL+"/pw-api/mounts/notify", bytes.NewReader(body))
		if err != nil {
			return
		}
		mac := hmac.New(sha256.New, []byte(s.cfg.PwikitSecret))
		mac.Write(body)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Pwikit-Mount", s.cfg.PwikitMount)
		req.Header.Set("X-Pwikit-Signature", hex.EncodeToString(mac.Sum(nil)))
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			slog.Warn("notify pwikit", "err", err)
			return
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusNoContent {
			slog.Warn("notify pwikit", "status", resp.StatusCode)
		}
	}()
}
