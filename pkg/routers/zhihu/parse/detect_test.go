package parse

import (
	"errors"
	"io"
	"testing"
)

// fakeAI implements ai.AI for detector tests; only Classify is exercised.
type fakeAI struct {
	reply string
	err   error
	calls int
}

func (f *fakeAI) Polish(text string) (string, error)        { return text, nil }
func (f *fakeAI) Text(io.Reader) (string, error)            { return "", nil }
func (f *fakeAI) Conclude(text string) (string, error)      { return text, nil }
func (f *fakeAI) TranslateToZh(text string) (string, error) { return text, nil }
func (f *fakeAI) Embed(text string) ([]float32, error)      { return nil, nil }
func (f *fakeAI) Classify(prompt string) (string, error) {
	f.calls++
	return f.reply, f.err
}

const testAuthor = "test-author"

func newTestDetector(reply string, err error) (*ContentDetector, *fakeAI) {
	f := &fakeAI{reply: reply, err: err}
	d := &ContentDetector{
		ai:       f,
		criteria: map[string]string{testAuthor: "test criteria"},
	}
	return d, f
}

func TestDetect_Hit(t *testing.T) {
	d, _ := newTestDetector(`{"skip": true, "reason": "ad content"}`, nil)
	res, detected, err := d.Detect(testAuthor, "some text")
	if err != nil || !detected {
		t.Fatalf("want detected,no-err; got detected=%v err=%v", detected, err)
	}
	if !res.Skip || res.Reason != "ad content" {
		t.Fatalf("want skip+reason; got %+v", res)
	}
}

func TestDetect_NonHit(t *testing.T) {
	d, _ := newTestDetector(`{"skip": false, "reason": ""}`, nil)
	res, detected, err := d.Detect(testAuthor, "some text")
	if err != nil || !detected {
		t.Fatalf("want detected,no-err; got detected=%v err=%v", detected, err)
	}
	if res.Skip {
		t.Fatalf("want no skip; got %+v", res)
	}
}

func TestDetect_HitWithCodeFence(t *testing.T) {
	d, _ := newTestDetector("```json\n{\"skip\": true, \"reason\": \"x\"}\n```", nil)
	res, detected, err := d.Detect(testAuthor, "some text")
	if err != nil || !detected || !res.Skip {
		t.Fatalf("want fenced JSON parsed as hit; got detected=%v err=%v res=%+v", detected, err, res)
	}
}

func TestDetect_MalformedJSON_FailOpen(t *testing.T) {
	d, _ := newTestDetector("not json at all", nil)
	_, detected, err := d.Detect(testAuthor, "some text")
	// detected=true so the caller records DetectStatusFailed (fail-open, not skipped).
	if !detected || err == nil {
		t.Fatalf("want detected=true with error (fail-open); got detected=%v err=%v", detected, err)
	}
}

func TestDetect_AIError_FailOpen(t *testing.T) {
	d, f := newTestDetector("", errors.New("boom"))
	_, detected, err := d.Detect(testAuthor, "some text")
	if !detected || err == nil {
		t.Fatalf("want detected=true with error (fail-open); got detected=%v err=%v", detected, err)
	}
	if f.calls != detectMaxRetry {
		t.Fatalf("want %d retries on persistent error; got %d", detectMaxRetry, f.calls)
	}
}

func TestDetect_UnregisteredAuthor(t *testing.T) {
	d, f := newTestDetector(`{"skip": true}`, nil)
	res, detected, err := d.Detect("someone-else", "some text")
	if detected || err != nil || res.Skip {
		t.Fatalf("want detected=false,no-err,no-skip for unregistered; got detected=%v err=%v res=%+v", detected, err, res)
	}
	if f.calls != 0 {
		t.Fatalf("want no AI call for unregistered author; got %d", f.calls)
	}
}
