package xratelimit

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
)

func newTestGuard(t *testing.T, cfg Config) (*IPBanGuard, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return NewIPBanGuard(rdb, nil, cfg), mr
}

// driveScan fires n GET requests for `path` from `ip` through the guard and
// returns the status code of the last request.
func driveScan(t *testing.T, g *IPBanGuard, ip, path string, n int, authHeader string) int {
	t.Helper()
	e := echo.New()
	h := g.Middleware()(func(c echo.Context) error {
		return c.NoContent(http.StatusNotFound)
	})
	var last int
	for i := 0; i < n; i++ {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set(echo.HeaderXRealIP, ip)
		if authHeader != "" {
			req.Header.Set(echo.HeaderAuthorization, authHeader)
		}
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		_ = h(c)
		last = rec.Code
	}
	return last
}

func TestIPBan_BansAfterThreshold(t *testing.T) {
	cfg := Config{Threshold: 5, WindowTTL: 10 * time.Minute, BanDuration: time.Hour}
	g, _ := newTestGuard(t, cfg)
	ip := "179.43.146.227"

	// First 4 scan hits: not yet banned (handler still runs -> 404).
	if code := driveScan(t, g, ip, "/redirect.php", 4, ""); code != http.StatusNotFound {
		t.Fatalf("after 4 hits code=%d, want 404 (not yet banned)", code)
	}
	// 5th hit crosses threshold and bans for subsequent requests.
	// The 5th request itself still runs (scoring happens post-handler), but the
	// 6th must be blocked with 403.
	driveScan(t, g, ip, "/redirect.php", 1, "") // 5th -> triggers ban
	if code := driveScan(t, g, ip, "/redirect.php", 1, ""); code != http.StatusForbidden {
		t.Fatalf("after threshold code=%d, want 403 (banned)", code)
	}
}

func TestIPBan_AuthenticatedNeverBanned(t *testing.T) {
	cfg := Config{Threshold: 3, WindowTTL: 10 * time.Minute, BanDuration: time.Hour}
	g, _ := newTestGuard(t, cfg)
	ip := "10.0.0.9"

	// 20 scan-looking hits but WITH an auth header -> never scored, never banned.
	code := driveScan(t, g, ip, "/redirect.php", 20, "Bearer faketoken")
	if code != http.StatusNotFound {
		t.Fatalf("authenticated code=%d, want 404 (never banned)", code)
	}
	banned, _ := g.IsBanned(httptest.NewRequest(http.MethodGet, "/", nil).Context(), ip)
	if banned {
		t.Fatal("authenticated IP must not be banned")
	}
}

func TestIPBan_NormalPathNotScored(t *testing.T) {
	cfg := Config{Threshold: 2, WindowTTL: 10 * time.Minute, BanDuration: time.Hour}
	g, _ := newTestGuard(t, cfg)
	ip := "123.22.244.126" // a real VN user IP from logs

	// Hitting legit (non-scanner) paths many times must never ban, even on 404.
	code := driveScan(t, g, ip, "/secret/99999", 30, "")
	if code != http.StatusNotFound {
		t.Fatalf("normal path code=%d, want 404 (not banned)", code)
	}
	banned, _ := g.IsBanned(httptest.NewRequest(http.MethodGet, "/", nil).Context(), ip)
	if banned {
		t.Fatal("user hitting normal 404s must not be banned")
	}
}

func TestIPBan_Unban(t *testing.T) {
	cfg := Config{Threshold: 1, WindowTTL: time.Minute, BanDuration: time.Hour}
	g, _ := newTestGuard(t, cfg)
	ip := "1.2.3.4"
	ctx := httptest.NewRequest(http.MethodGet, "/", nil).Context()

	driveScan(t, g, ip, "/.env", 1, "") // threshold 1 -> banned immediately
	if banned, _ := g.IsBanned(ctx, ip); !banned {
		t.Fatal("expected banned after 1 hit")
	}
	if err := g.Unban(ctx, ip); err != nil {
		t.Fatalf("unban: %v", err)
	}
	if banned, _ := g.IsBanned(ctx, ip); banned {
		t.Fatal("expected unbanned after Unban")
	}
}

func TestIPBan_BanExpires(t *testing.T) {
	cfg := Config{Threshold: 1, WindowTTL: time.Minute, BanDuration: 2 * time.Second}
	g, mr := newTestGuard(t, cfg)
	ip := "5.6.7.8"
	ctx := httptest.NewRequest(http.MethodGet, "/", nil).Context()

	driveScan(t, g, ip, "/wp-login.php", 1, "")
	if banned, _ := g.IsBanned(ctx, ip); !banned {
		t.Fatal("expected banned")
	}
	mr.FastForward(3 * time.Second) // advance past BanDuration
	if banned, _ := g.IsBanned(ctx, ip); banned {
		t.Fatal("expected ban to expire")
	}
}

func TestIPBan_ListBanned(t *testing.T) {
	cfg := Config{Threshold: 1, WindowTTL: time.Minute, BanDuration: time.Hour}
	g, _ := newTestGuard(t, cfg)
	ctx := httptest.NewRequest(http.MethodGet, "/", nil).Context()

	driveScan(t, g, "8.8.8.8", "/.env", 1, "")
	driveScan(t, g, "9.9.9.9", "/phpmyadmin/index.php", 1, "")
	list, err := g.ListBanned(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("listed %d banned, want 2: %v", len(list), list)
	}
}

// TestIPBan_LegitAppPathsNeverMatch guards against regex false-positives: real
// app paths (API, FE routes, share links) must never be scored as scanner
// probes, otherwise an un-authenticated visitor could get banned. If a future
// route would collide with scanPathRe, this test fails first.
func TestIPBan_LegitAppPathsNeverMatch(t *testing.T) {
	legit := []string{
		"/", "/login", "/dashboard", "/admin", "/data", "/items",
		"/api/v1/auth/login", "/api/v1/items", "/api/v1/items/123",
		"/api/v1/redirect-after-pay", "/api/v1/url-shortener", "/share/out-link",
		"/assets/index-abc123.js", "/config.json", "/favicon.ico",
		"/api/v1/things/url", "/redirect", "/out", "/url",
	}
	for _, p := range legit {
		if scanPathRe.MatchString(p) {
			t.Errorf("legit path %q wrongly matched scanPathRe (would penalise real users)", p)
		}
	}

	malicious := []string{
		"/redirect.php", "/out.php", "/index.php", "/.env", "/wp-admin/",
		"/.git/config", "/phpmyadmin/", "/cgi-bin/test", "/actuator/health",
	}
	for _, p := range malicious {
		if !scanPathRe.MatchString(p) {
			t.Errorf("malicious path %q should match scanPathRe but did not", p)
		}
	}
}
