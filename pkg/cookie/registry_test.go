package cookie

import "testing"

func TestBrowserSpecByNameDomainRequiresDomainAndRejectsManualCredential(t *testing.T) {
	tests := []struct {
		name       string
		cookieName string
		domain     string
		want       bool
	}{
		{name: "matching browser cookie", cookieName: "token", domain: ".xiaobot.net", want: true},
		{name: "missing domain", cookieName: "token", domain: ""},
		{name: "wrong domain", cookieName: "token", domain: ".example.com"},
		{name: "manual github token", cookieName: "access_token", domain: ".github.com"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, got := BrowserSpecByNameDomain(tc.cookieName, tc.domain)
			if got != tc.want {
				t.Fatalf("BrowserSpecByNameDomain(%q, %q) found = %v, want %v", tc.cookieName, tc.domain, got, tc.want)
			}
		})
	}
}

func TestSpecByPlatformNameDisambiguatesCredential(t *testing.T) {
	spec, ok := SpecByPlatformName("github", "access_token")
	if !ok || spec.Type != CookieTypeGitHubAccessToken || spec.Kind() != "token" || spec.UpdateMethod() != "api" {
		t.Fatalf("GitHub credential spec = %+v, found = %v", spec, ok)
	}

	spec, ok = SpecByPlatformName("xiaobot", "token")
	if !ok || spec.Type != CookieTypeXiaobotAccessToken || spec.Kind() != "browser_cookie" || spec.UpdateMethod() != "browser_cookie_import" {
		t.Fatalf("Xiaobot credential spec = %+v, found = %v", spec, ok)
	}
}
