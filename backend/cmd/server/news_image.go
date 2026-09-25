package main

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

const maxNewsImageBytes int64 = 15 << 20

func (s *server) newsArticleImage(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id < 1 {
		jsonErr(w, http.StatusNotFound, "article image not found")
		return
	}
	visibility := "a.is_hidden=0"
	if s.isAdminRequest(r) {
		visibility = "1=1"
	}
	var imageURL string
	var hidden int
	err = s.db.QueryRowContext(r.Context(), `SELECT a.image_url,a.is_hidden FROM articles a JOIN sources s ON s.id=a.source_id WHERE a.id=? AND a.thread_post_id='' AND a.image_url<>'' AND s.enabled=1 AND `+visibility, id).Scan(&imageURL, &hidden)
	if errors.Is(err, sql.ErrNoRows) {
		jsonErr(w, http.StatusNotFound, "article image not found")
		return
	}
	if err != nil {
		jsonErr(w, http.StatusInternalServerError, "could not load article image")
		return
	}

	proxyNewsImage(w, r, imageURL, hidden == 0)
}

func proxyNewsImage(w http.ResponseWriter, r *http.Request, imageURL string, cacheable bool) {
	if !allowedNewsImageURL(imageURL) {
		jsonErr(w, http.StatusBadGateway, "article image source is unavailable")
		return
	}
	client := &http.Client{
		Timeout:   20 * time.Second,
		Transport: newsImageTransport(),
		CheckRedirect: func(next *http.Request, via []*http.Request) error {
			if len(via) >= 3 || !allowedNewsImageURL(next.URL.String()) {
				return errors.New("unsupported image redirect")
			}
			if len(via) > 0 && via[len(via)-1].URL.Scheme == "https" && next.URL.Scheme != "https" {
				return errors.New("insecure image redirect")
			}
			return nil
		},
	}
	defer client.CloseIdleConnections()
	proxyNewsImageWithClient(w, r, imageURL, cacheable, client)
}

func newsImageTransport() *http.Transport {
	return &http.Transport{
		Proxy:                 nil,
		DialContext:           dialPublicImageHost,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
	}
}

func proxyNewsImageWithClient(w http.ResponseWriter, r *http.Request, imageURL string, cacheable bool, client *http.Client) {
	if !allowedNewsImageURL(imageURL) {
		jsonErr(w, http.StatusBadGateway, "article image source is unavailable")
		return
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, imageURL, nil)
	if err != nil {
		jsonErr(w, http.StatusBadGateway, "article image source is unavailable")
		return
	}
	req.Header.Set("Accept", "image/avif,image/webp,image/apng,image/svg+xml,image/*;q=0.8,*/*;q=0.5")
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; TelegramNewsImageProxy/1.0)")
	res, err := client.Do(req)
	if err != nil {
		jsonErr(w, http.StatusBadGateway, "article image source is unavailable")
		return
	}
	defer res.Body.Close()
	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		jsonErr(w, http.StatusBadGateway, "article image source is unavailable")
		return
	}
	contentType, _, err := mime.ParseMediaType(res.Header.Get("Content-Type"))
	contentType = strings.ToLower(contentType)
	if err != nil || !strings.HasPrefix(contentType, "image/") || res.ContentLength > maxNewsImageBytes {
		jsonErr(w, http.StatusBadGateway, "article image source is unavailable")
		return
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, maxNewsImageBytes+1))
	if err != nil || len(data) == 0 || int64(len(data)) > maxNewsImageBytes {
		jsonErr(w, http.StatusBadGateway, "article image source is unavailable")
		return
	}
	w.Header().Set("Content-Type", contentType)
	if cacheable {
		w.Header().Set("Cache-Control", "public, max-age=3600, stale-while-revalidate=86400")
	} else {
		w.Header().Set("Cache-Control", "private, no-store")
	}
	w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func allowedNewsImageURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.User != nil || u.Hostname() == "" || strings.Contains(u.Hostname(), "%") {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	if ip := net.ParseIP(u.Hostname()); ip != nil && !isPublicNewsImageIP(ip) {
		return false
	}
	port := u.Port()
	return port == "" || port == "80" || port == "443"
}

func dialPublicImageHost(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil || (port != "80" && port != "443") {
		return nil, errors.New("unsupported image host")
	}
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, strings.TrimSuffix(host, "."))
	if err != nil {
		return nil, err
	}
	dialer := net.Dialer{Timeout: 8 * time.Second, KeepAlive: 30 * time.Second}
	var lastErr error
	for _, resolved := range ips {
		if !isPublicNewsImageIP(resolved.IP) {
			continue
		}
		conn, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(resolved.IP.String(), port))
		if dialErr == nil {
			return conn, nil
		}
		lastErr = dialErr
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, errors.New("image host did not resolve to a public address")
}

func isPublicNewsImageIP(ip net.IP) bool {
	if ipv4 := ip.To4(); ipv4 != nil {
		ip = ipv4
	}
	if ip == nil || !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return false
	}
	reserved := []string{"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "192.88.99.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4", "64:ff9b::/96", "64:ff9b:1::/48", "100::/64", "2001::/23", "2001:db8::/32", "2001:10::/28"}
	for _, block := range reserved {
		_, network, err := net.ParseCIDR(block)
		if err == nil && network.Contains(ip) {
			return false
		}
	}
	return true
}
