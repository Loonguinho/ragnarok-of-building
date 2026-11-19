package web

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// Builds can be shared as a paste on paste.rs (anonymous, no key), like
// Path of Building's pastebin links: the paste holds the build summary and
// its share code, and pasting the paste's link into Import reads it back.

const defaultPasteURL = "https://paste.rs"

// maxPasteText caps the summary sent along with the code.
const maxPasteText = 8 << 10

var (
	pasteID     = regexp.MustCompile(`^/([A-Za-z0-9]{1,32})(?:\.[A-Za-z0-9]{1,8})?$`)
	codeInLink  = regexp.MustCompile(`[?&]b=([A-Za-z0-9_-]+)`)
	codeInPaste = regexp.MustCompile(`\b(z[A-Za-z0-9_-]{16,}|eyJ[A-Za-z0-9_-]{16,})`)
)

var pasteHTTP = &http.Client{Timeout: 15 * time.Second}

func (s *Server) pasteBase() string {
	if s.opt.PasteURL != "" {
		return strings.TrimSuffix(s.opt.PasteURL, "/")
	}
	return defaultPasteURL
}

// handlePasteCreate (POST b, text) pastes the build and answers {"url": ...}.
func (s *Server) handlePasteCreate(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBuildBody+maxPasteText+1024)
	code := r.FormValue("b")
	if _, ok := decodeBuild(code); !ok {
		http.Error(w, "invalid build code", http.StatusBadRequest)
		return
	}
	text := strings.TrimSpace(r.FormValue("text"))
	if len(text) > maxPasteText {
		text = text[:maxPasteText]
	}
	// The code always goes in, on its own line, so Import finds it.
	body := text + "\n\n" + code + "\n"
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, s.pasteBase()+"/", strings.NewReader(body))
	req.Header.Set("Content-Type", "text/plain; charset=utf-8")
	req.Header.Set("User-Agent", "ragnarok-of-building")
	res, err := pasteHTTP.Do(req)
	if err != nil {
		s.opt.Log.Warn("paste", "err", err)
		http.Error(w, "paste service unreachable", http.StatusBadGateway)
		return
	}
	defer res.Body.Close()
	out, _ := io.ReadAll(io.LimitReader(res.Body, 1024))
	link := strings.TrimSpace(string(out))
	// 201: stored whole (206 would be a truncated paste: useless).
	if res.StatusCode != http.StatusCreated || !s.isPasteLink(link) {
		s.opt.Log.Warn("paste", "status", res.StatusCode, "body", link)
		http.Error(w, "paste service refused", http.StatusBadGateway)
		return
	}
	writeJSON(w, map[string]string{"url": link})
}

// isPasteLink reports whether u is a paste on the paste service.
func (s *Server) isPasteLink(u string) bool {
	_, err := s.pasteRawURL(u)
	return err == nil
}

// pasteRawURL turns a paste link into the URL of its raw text. Only links
// to the paste service are accepted (the server never fetches other hosts).
func (s *Server) pasteRawURL(link string) (string, error) {
	link = strings.TrimSpace(link)
	if !strings.Contains(link, "://") {
		link = "https://" + link // "paste.rs/Ab12"
	}
	u, err := url.Parse(link)
	if err != nil {
		return "", err
	}
	base, _ := url.Parse(s.pasteBase())
	if u.Host != base.Host || (u.Scheme != "https" && u.Scheme != base.Scheme) {
		return "", errors.New("not a paste link")
	}
	m := pasteID.FindStringSubmatch(u.Path)
	if m == nil {
		return "", errors.New("not a paste link")
	}
	return s.pasteBase() + "/" + m[1], nil
}

// handlePasteRead (GET u=paste link) answers {"code": ...}: the build code
// found in the paste.
func (s *Server) handlePasteRead(w http.ResponseWriter, r *http.Request) {
	raw, err := s.pasteRawURL(r.URL.Query().Get("u"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	req.Header.Set("User-Agent", "ragnarok-of-building")
	res, err := pasteHTTP.Do(req)
	if err != nil {
		http.Error(w, "paste service unreachable", http.StatusBadGateway)
		return
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		http.Error(w, fmt.Sprintf("paste not found (%d)", res.StatusCode), http.StatusNotFound)
		return
	}
	text, _ := io.ReadAll(io.LimitReader(res.Body, maxBuildBody*2))
	code := findBuildCode(text)
	if code == "" {
		http.Error(w, "no build code in the paste", http.StatusUnprocessableEntity)
		return
	}
	writeJSON(w, map[string]string{"code": code})
}

// findBuildCode returns the first valid build code in a text: from a
// simulator link (?b=...) or a bare code.
func findBuildCode(text []byte) string {
	var cands [][]byte
	for _, m := range codeInLink.FindAllSubmatch(text, -1) {
		cands = append(cands, m[1])
	}
	for _, m := range codeInPaste.FindAllSubmatch(text, -1) {
		cands = append(cands, m[1])
	}
	for _, c := range cands {
		if _, ok := decodeBuild(string(c)); ok {
			return string(c)
		}
	}
	return ""
}

func writeJSON(w http.ResponseWriter, v any) {
	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(v)
	w.Header().Set("Content-Type", "application/json")
	w.Write(buf.Bytes())
}
