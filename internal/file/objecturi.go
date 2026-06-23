package file

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// ErrNoStorageProvider is returned by ObjectURI when an object has no storage
// provider recorded, so callers can distinguish it from a real error.
var ErrNoStorageProvider = errors.New("object has no storage provider")

// ObjectURI builds the public URL for a stored object: the first storage
// provider joined with the object key, each path segment PathEscaped while "/"
// separators are preserved (keys without special characters stay byte-identical;
// a non-ASCII filename segment gets escaped). Returns ErrNoStorageProvider when
// no provider is recorded.
func ObjectURI(storageProvider []string, objectKey string) (string, error) {
	if len(storageProvider) == 0 {
		return "", fmt.Errorf("%w: object_key=%s", ErrNoStorageProvider, objectKey)
	}
	segs := strings.Split(objectKey, "/")
	for i, s := range segs {
		segs[i] = url.PathEscape(s)
	}
	return storageProvider[0] + "/" + strings.Join(segs, "/"), nil
}
