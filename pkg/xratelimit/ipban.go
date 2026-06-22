// Package xratelimit provides Redis-backed abuse protection middlewares.
//
// IPBanGuard auto-bans IPs that exhibit vulnerability-scanner behaviour:
// hitting many obviously-malicious paths (PHP/WP/.env/panel probes) or
// generating a burst of 404s on suspicious paths. State lives in Redis so the
// ban is shared across all backend instances and survives a single instance
// restart.
//
// Design notes:
//   - Two Redis keys per offender IP:
//     ban:{ip}   -> set when banned, TTL = BanDuration. Presence = blocked.
//     scan:{ip}  -> sliding-window counter (INCR + EXPIRE WindowTTL).
//   - A request is "suspicious" when it matches the scan path regex. Such a
//     request increments scan:{ip}; once it reaches Threshold the IP is banned.
//   - Requests carrying an Authorization header are treated as authenticated
//     traffic and are never scored/banned (real users, even if they mistype a
//     URL, won't get caught). The guard is intentionally cheap (no JWT verify).
//   - Banned IPs are rejected up-front with 403 before any handler runs.
//   - Redis failures fail OPEN (never block legitimate traffic on infra issues).
package xratelimit

import (
	"context"
	"regexp"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"

	"vtv.vn/backend/pkg/xhttp"
	"vtv.vn/backend/pkg/xlogger"
)

const (
	banKeyPrefix  = "app:ipban:ban:"
	scanKeyPrefix = "app:ipban:scan:"
)

// scanPathRe matches request paths typical of automated vulnerability scanners.
// Mirrors (and extends) the nginx bad_path block so paths that slip past nginx
// are still caught here.
//
// IMPORTANT: only unambiguously-malicious patterns are listed. We deliberately
// do NOT match bare words like /redirect, /url or /out (which a future
// legitimate route or share link could use) because the bot variants
// (/redirect.php, /out.php ...) are already caught by the \.php rule. Keeping
// the set tight guarantees real users are never scored by accident.
var scanPathRe = regexp.MustCompile(`(?i)(\.php($|\?|/)|/wp-admin|/wp-login|/wp-content|/wp-includes|/xmlrpc|/vendor/phpunit|/eval-stdin|/\.env|/\.git|/phpmyadmin|/\.aws|/cpanel|/whm|/webmail|/plesk|/___proxy_subdomain|/cgi-bin|/boaform|/manager/html|/solr/|/actuator|/console/|/struts|/hudson|/jenkins|/\.well-known/.*\.php|/containers/json)`)

// Config tunes the ban behaviour.
type Config struct {
	Threshold   int           // suspicious hits within the window before banning
	WindowTTL   time.Duration // sliding window for counting suspicious hits
	BanDuration time.Duration // how long an IP stays banned
}

// DefaultConfig: 5 scan hits within 10 minutes -> banned for 24h.
func DefaultConfig() Config {
	return Config{
		Threshold:   5,
		WindowTTL:   10 * time.Minute,
		BanDuration: 24 * time.Hour,
	}
}

// IPBanGuard is a Redis-backed scanner auto-ban middleware.
type IPBanGuard struct {
	rdb    *redis.Client
	log    *xlogger.Logger
	cfg    Config
	bypass func(c echo.Context) bool // returns true to skip scoring/banning
}

// NewIPBanGuard builds the guard. A nil logger is tolerated (logging skipped).
func NewIPBanGuard(rdb *redis.Client, log *xlogger.Logger, cfg Config) *IPBanGuard {
	if cfg.Threshold <= 0 {
		cfg.Threshold = DefaultConfig().Threshold
	}
	if cfg.WindowTTL <= 0 {
		cfg.WindowTTL = DefaultConfig().WindowTTL
	}
	if cfg.BanDuration <= 0 {
		cfg.BanDuration = DefaultConfig().BanDuration
	}
	return &IPBanGuard{
		rdb: rdb,
		log: log,
		cfg: cfg,
		// Authenticated requests (carry a bearer token) bypass scoring so real
		// users are never auto-banned for mistyped URLs.
		bypass: func(c echo.Context) bool {
			return c.Request().Header.Get(echo.HeaderAuthorization) != ""
		},
	}
}

// Middleware returns the Echo middleware. It must run BEFORE auth so banned IPs
// are rejected as early as possible, and it inspects the response status after
// the handler runs to score suspicious 404s.
func (g *IPBanGuard) Middleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ip := c.RealIP()
			ctx := c.Request().Context()

			// 1) Already banned? Reject immediately.
			if g.isBanned(ctx, ip) {
				return xhttp.AppErrorResponse(c, xhttp.ForbiddenErrorf("truy cập bị chặn"))
			}

			// 2) Let the request run.
			err := next(c)

			// 3) Skip scoring for authenticated traffic.
			if g.bypass != nil && g.bypass(c) {
				return err
			}

			// 4) Score suspicious requests. A request is suspicious if its path
			// looks like a scanner probe. (We rely on the path pattern rather
			// than raw 404 count so a real user browsing to a valid-but-missing
			// resource isn't penalised.)
			path := c.Request().URL.Path
			if scanPathRe.MatchString(path) {
				g.score(ctx, ip, path)
			}
			return err
		}
	}
}

func (g *IPBanGuard) isBanned(ctx context.Context, ip string) bool {
	n, err := g.rdb.Exists(ctx, banKeyPrefix+ip).Result()
	if err != nil {
		return false // fail open
	}
	return n > 0
}

// score increments the sliding-window counter for ip and bans it once the
// threshold is crossed. Uses a pipeline so INCR+EXPIRE are a single round-trip.
func (g *IPBanGuard) score(ctx context.Context, ip, path string) {
	key := scanKeyPrefix + ip
	pipe := g.rdb.TxPipeline()
	incr := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, g.cfg.WindowTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		return // fail open
	}
	count := incr.Val()
	if count < int64(g.cfg.Threshold) {
		return
	}
	// Threshold crossed -> ban. SET with TTL is idempotent if already banned.
	if err := g.rdb.Set(ctx, banKeyPrefix+ip, 1, g.cfg.BanDuration).Err(); err != nil {
		return
	}
	// Clear the counter so it restarts cleanly if the ban later expires.
	g.rdb.Del(ctx, key)
	if g.log != nil {
		g.log.Warn("ip auto-banned: scanner behaviour",
			xlogger.String("ip", ip),
			xlogger.String("trigger_path", path),
			xlogger.Int("scan_hits", int(count)),
			xlogger.Dur("ban_duration", g.cfg.BanDuration),
		)
	}
}

// --- Admin helpers (for an ops endpoint) ---

// IsBanned reports whether ip is currently banned.
func (g *IPBanGuard) IsBanned(ctx context.Context, ip string) (bool, error) {
	n, err := g.rdb.Exists(ctx, banKeyPrefix+ip).Result()
	return n > 0, err
}

// Unban removes a ban (and any pending counter) for ip.
func (g *IPBanGuard) Unban(ctx context.Context, ip string) error {
	return g.rdb.Del(ctx, banKeyPrefix+ip, scanKeyPrefix+ip).Err()
}

// ListBanned returns currently banned IPs (scans ban:* keys). For ops/debug
// only; uses SCAN to avoid blocking Redis.
func (g *IPBanGuard) ListBanned(ctx context.Context) ([]string, error) {
	var out []string
	iter := g.rdb.Scan(ctx, 0, banKeyPrefix+"*", 100).Iterator()
	for iter.Next(ctx) {
		out = append(out, iter.Val()[len(banKeyPrefix):])
	}
	return out, iter.Err()
}
