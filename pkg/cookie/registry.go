package cookie

import (
	"strings"
	"time"

	"go.uber.org/zap"
)

// Spec is the single source of truth for one cookie: how it is matched on write,
// how long it should live, and (optionally) how to validate it. Everything that is
// platform-specific about a cookie lives here and nowhere else. Storage still keys
// by the int Type, so no DB migration is involved.
type Spec struct {
	Type       int           // existing CookieTypeXxx; storage key (unchanged)
	Platform   string        // "zhihu"
	Name       string        // browser cookie name; match key on write
	Domains    []string      // e.g. [".zhihu.com"]; tie-breaker when a Name is ambiguous
	SafetyGap  time.Duration // subtracted from the real expiry before storing
	DefaultTTL time.Duration // used when the incoming cookie carries no expiry; 0 => package DefaultTTL
	Manual     bool          // true => not browser-grabbed (e.g. github PAT, hand-entered)
}

// Probe validates a candidate cookie value (e.g. by making a test request). It is
// registered at startup via RegisterProbe rather than embedded in the registry, so
// pkg/cookie need not import the platform request packages (which import pkg/cookie).
type Probe func(value string, l *zap.Logger) error

var registry = []Spec{
	{Type: CookieTypeZhihuDC0, Platform: "zhihu", Name: "d_c0", Domains: []string{".zhihu.com"}, SafetyGap: 24 * time.Hour},
	{Type: CookieTypeZhihuZC0, Platform: "zhihu", Name: "z_c0", Domains: []string{".zhihu.com"}, SafetyGap: 24 * time.Hour},
	{Type: CookieTypeZhihuZSECK, Platform: "zhihu", Name: "__zse_ck", Domains: []string{".zhihu.com"}, SafetyGap: 48 * time.Hour},
	{Type: CookieTypeZsxqAccessToken, Platform: "zsxq", Name: "zsxq_access_token", Domains: []string{".zsxq.com"}, SafetyGap: time.Hour},
	{Type: CookieTypeXiaobotAccessToken, Platform: "xiaobot", Name: "token", Domains: []string{".xiaobot.net"}},
	{Type: CookieTypeGitHubAccessToken, Platform: "github", Name: "access_token", SafetyGap: 24 * time.Hour, Manual: true},
}

// probes holds validators registered at startup. It is written once during server
// wiring (single-threaded) and only read afterwards, so it needs no lock.
var probes = map[int]Probe{}

// RegisterProbe attaches a validator to a cookie type. Call during startup wiring.
func RegisterProbe(cookieType int, p Probe) { probes[cookieType] = p }

// ProbeFor returns the validator for a cookie type, or nil if none is registered.
func ProbeFor(cookieType int) Probe { return probes[cookieType] }

// SpecByNameDomain resolves an incoming browser cookie to its Spec by name, using
// domain only to disambiguate when more than one Spec shares the same Name. domain
// may be empty.
func SpecByNameDomain(name, domain string) (Spec, bool) {
	var matches []Spec
	for _, s := range registry {
		if s.Name == name {
			matches = append(matches, s)
		}
	}
	switch len(matches) {
	case 0:
		return Spec{}, false
	case 1:
		return matches[0], true
	default:
		for _, s := range matches {
			if domainMatches(s.Domains, domain) {
				return s, true
			}
		}
		return Spec{}, false
	}
}

func domainMatches(domains []string, domain string) bool {
	d := strings.TrimPrefix(domain, ".")
	for _, want := range domains {
		w := strings.TrimPrefix(want, ".")
		if d == w || strings.HasSuffix(d, "."+w) {
			return true
		}
	}
	return false
}

// SpecsByPlatform returns every Spec belonging to a platform, in registry order.
func SpecsByPlatform(platform string) []Spec {
	var out []Spec
	for _, s := range registry {
		if s.Platform == platform {
			out = append(out, s)
		}
	}
	return out
}

// SpecByType returns the Spec for a cookie type.
func SpecByType(cookieType int) (Spec, bool) {
	for _, s := range registry {
		if s.Type == cookieType {
			return s, true
		}
	}
	return Spec{}, false
}

// AllSpecs returns all registered specs in registry order.
func AllSpecs() []Spec { return registry }

// Label is a human-readable "platform/name" identifier used in logs and notifications.
func (s Spec) Label() string { return s.Platform + "/" + s.Name }
