package tombkeeper

import "testing"

// RedownloadObject must overwrite the OSS object at the given key with the freshly
// downloaded bytes (the 0-byte backfill path), keeping the key unchanged so embedded
// markdown links keep working.
func TestRedownloadObjectOverwritesObjectKey(t *testing.T) {
	f := newFakeFile()
	f.saved["tombkeeper/abc.jpg"] = nil // simulate the pre-existing 0-byte object
	req := &fakeRequester{picAvailable: true}

	usedURL, err := RedownloadObject(req, f, "tombkeeper/abc.jpg", "abc", testLogger())
	if err != nil {
		t.Fatalf("RedownloadObject: %v", err)
	}
	if usedURL == "" {
		t.Error("expected a non-empty used URL")
	}
	if got := string(f.saved["tombkeeper/abc.jpg"]); got != "IMGDATA" {
		t.Errorf("object not overwritten with fresh bytes: got %q", got)
	}
}

// RedownloadObject must surface an error (not silently store an empty object) when
// every CDN candidate fails, so the migration counts it as a failure and retries.
func TestRedownloadObjectErrorsWhenAllCandidatesFail(t *testing.T) {
	f := newFakeFile()
	req := &fakeRequester{picAvailable: false}

	if _, err := RedownloadObject(req, f, "tombkeeper/abc.jpg", "abc", testLogger()); err == nil {
		t.Error("expected error when all candidates fail, got nil")
	}
}

func TestPicIDOf(t *testing.T) {
	cases := map[string]string{
		"53899d01ly1ie5wrym85ej20sg0gl41f":     "53899d01ly1ie5wrym85ej20sg0gl41f",
		"https://wx2.sinaimg.cn/large/abc.jpg": "abc",
		"  trimmed  ":                          "trimmed",
	}
	for in, want := range cases {
		if got := picIDOf(in); got != want {
			t.Errorf("picIDOf(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCandidateURLsBareID(t *testing.T) {
	cands, orig := candidateURLs("abc123")
	if orig != "https://wx1.sinaimg.cn/large/abc123.jpg" {
		t.Errorf("original = %s", orig)
	}
	if len(cands) != 16 { // 12 sina hosts + 4 third-party proxies
		t.Fatalf("candidate count = %d, want 16", len(cands))
	}
	if cands[0] != "https://wx1.sinaimg.cn/large/abc123.jpg" {
		t.Errorf("first candidate = %s", cands[0])
	}
}

func TestImageURLAllowed(t *testing.T) {
	allow := []string{
		"https://wx2.sinaimg.cn/large/a.jpg",
		"https://i0.wp.com/wx2.sinaimg.cn/large/a.jpg",
		"https://cdn.ipfsscan.io/weibo/large/a.jpg",
	}
	reject := []string{
		"http://169.254.169.254/latest/meta-data",
		"https://evil.example.com/a.jpg",
		"http://localhost/x",
		"ftp://wx2.sinaimg.cn/a.jpg",
		"https://notsinaimg.cn.evil.com/a.jpg",
	}
	for _, u := range allow {
		if !imageURLAllowed(u) {
			t.Errorf("should allow %s", u)
		}
	}
	for _, u := range reject {
		if imageURLAllowed(u) {
			t.Errorf("should reject %s", u)
		}
	}
}

func TestCandidateURLsRejectsUntrustedHost(t *testing.T) {
	untrusted := "http://169.254.169.254/latest/meta-data"
	cands, orig := candidateURLs(untrusted)
	if len(cands) != 0 {
		t.Errorf("untrusted host should yield no fetch candidates, got %v", cands)
	}
	if orig != untrusted {
		t.Errorf("original link should be preserved, got %s", orig)
	}
}

func TestCandidateURLsFullURL(t *testing.T) {
	full := "https://wx2.sinaimg.cn/large/abc.jpg"
	cands, orig := candidateURLs(full)
	if orig != full {
		t.Errorf("original = %s, want %s", orig, full)
	}
	if cands[0] != full {
		t.Errorf("first candidate = %s, want %s", cands[0], full)
	}
}
