package db

import (
	"errors"
	"strings"
	"testing"
)

func TestObjectURI(t *testing.T) {
	const provider = "https://oss.darkeli.com/rss"

	t.Run("image key stays byte-identical", func(t *testing.T) {
		o := &Object{StorageProvider: []string{provider}, ObjectKey: "zsxq/181111128155122.jpg"}
		got, err := o.URI()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := provider + "/zsxq/181111128155122.jpg"
		if got != want {
			t.Fatalf("got %q want %q", got, want)
		}
	})

	t.Run("file key escapes filename segment but preserves slash", func(t *testing.T) {
		o := &Object{StorageProvider: []string{provider}, ObjectKey: "zsxq/118558458122242-东昌：基因.docx"}
		got, err := o.URI()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Separator preserved.
		if !strings.HasPrefix(got, provider+"/zsxq/") {
			t.Fatalf("slash separator not preserved: %q", got)
		}
		// Special characters escaped (no raw fullwidth colon / CJK in output).
		if strings.ContainsAny(got, "：东昌") {
			t.Fatalf("filename segment not escaped: %q", got)
		}
		// "/" itself must not be percent-encoded.
		if strings.Contains(got, "%2F") || strings.Contains(got, "%2f") {
			t.Fatalf("path separator wrongly escaped: %q", got)
		}
	})

	t.Run("nil provider returns ErrNoStorageProvider", func(t *testing.T) {
		o := &Object{ObjectKey: "zsxq/1.jpg"}
		_, err := o.URI()
		if !errors.Is(err, ErrNoStorageProvider) {
			t.Fatalf("expected ErrNoStorageProvider, got %v", err)
		}
	})

	t.Run("empty provider slice returns ErrNoStorageProvider", func(t *testing.T) {
		o := &Object{StorageProvider: []string{}, ObjectKey: "zsxq/1.jpg"}
		_, err := o.URI()
		if !errors.Is(err, ErrNoStorageProvider) {
			t.Fatalf("expected ErrNoStorageProvider, got %v", err)
		}
	})
}
