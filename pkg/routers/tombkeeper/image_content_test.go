package tombkeeper

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

type observedImageStream struct {
	io.Reader
	closed bool
}

func (s *observedImageStream) Close() error { s.closed = true; return nil }
func TestImageValidationPreservesCloseAndReadError(t *testing.T) {
	original := &observedImageStream{Reader: bytes.NewReader(testPNG(t))}
	body, _, err := ValidateImageStream(original)
	if err != nil {
		t.Fatal(err)
	}
	if original.closed {
		t.Fatal("closed before caller finished")
	}
	if err = body.Close(); err != nil {
		t.Fatal(err)
	}
	if !original.closed {
		t.Fatal("underlying stream not closed")
	}
	boom := errors.New("storage unavailable")
	broken := &observedImageStream{Reader: imageErrorReader{boom}}
	_, _, err = ValidateImageStream(broken)
	if !errors.Is(err, boom) || errors.Is(err, ErrNotImage) {
		t.Fatalf("read error misclassified: %v", err)
	}
	if !broken.closed {
		t.Fatal("failed stream not closed")
	}
	invalid := &observedImageStream{Reader: bytes.NewReader([]byte("<html>error</html>"))}
	_, _, err = ValidateImageStream(invalid)
	if !errors.Is(err, ErrNotImage) || !invalid.closed {
		t.Fatalf("invalid content: %v", err)
	}
}

type imageErrorReader struct{ err error }

func (r imageErrorReader) Read([]byte) (int, error) { return 0, r.err }
