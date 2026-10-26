package weibochat

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"image/jpeg"
	"io"
	"os"

	"github.com/gen2brain/h265/heic"
)

// isHEIC checks HEVC brands in the ISO BMFF file-type box, not the HTTP header.
// Generic mif1 alone is insufficient: AVIF also uses that container brand.
func isHEIC(prefix []byte) bool {
	if len(prefix) < 16 || string(prefix[4:8]) != "ftyp" {
		return false
	}
	size := int(binary.BigEndian.Uint32(prefix[:4]))
	if size < 16 || size > len(prefix) || size%4 != 0 {
		return false
	}
	for offset := 8; offset < size; offset += 4 {
		if offset == 12 {
			continue
		} // minor version
		switch string(prefix[offset : offset+4]) {
		case "heic", "heix", "heim", "heis", "hevc", "hevx", "hevm", "hevs":
			return true
		}
	}
	return false
}

// storeHEIC retains the original bytes and creates a full-resolution browser
// rendition. Both uploads must succeed before the asset becomes stored.
func storeHEIC(ctx context.Context, files ObjectStore, reader io.Reader, length int64, keyPrefix string) (storedMedia, error) {
	original, err := os.CreateTemp("", "weibo-heic-*")
	if err != nil {
		return storedMedia{}, err
	}
	defer os.Remove(original.Name())
	defer original.Close()
	size, err := io.Copy(original, io.LimitReader(reader, maxAssetBytes+1))
	if err != nil {
		return storedMedia{}, fmt.Errorf("read HEIC: %w", err)
	}
	if size > maxAssetBytes {
		return storedMedia{}, fmt.Errorf("asset exceeds size limit")
	}
	if size == 0 || (length >= 0 && size != length) {
		return storedMedia{}, fmt.Errorf("incomplete asset stream")
	}
	if _, err = original.Seek(0, io.SeekStart); err != nil {
		return storedMedia{}, err
	}
	// Bound decoded memory and CPU independently of the compressed download size.
	decoded, err := heic.Decode(original, heic.Options{AutoRotate: true, FrameSizeLimit: 50_000_000, Threads: 2})
	if err != nil {
		return storedMedia{}, fmt.Errorf("decode HEIC: %w", err)
	}
	if err = ctx.Err(); err != nil {
		return storedMedia{}, err
	}
	var display bytes.Buffer
	if err = jpeg.Encode(&display, decoded, &jpeg.Options{Quality: 90}); err != nil {
		return storedMedia{}, fmt.Errorf("encode HEIC display: %w", err)
	}
	if _, err = original.Seek(0, io.SeekStart); err != nil {
		return storedMedia{}, err
	}
	key, displayKey := keyPrefix+".heic", keyPrefix+".display.jpg"
	if err = files.SaveStreamContext(ctx, key, io.NopCloser(original), size); err != nil {
		return storedMedia{}, fmt.Errorf("store HEIC original: %w", err)
	}
	if err = files.SaveStreamContext(ctx, displayKey, io.NopCloser(bytes.NewReader(display.Bytes())), int64(display.Len())); err != nil {
		return storedMedia{}, fmt.Errorf("store HEIC display: %w", err)
	}
	bounds := decoded.Bounds()
	return storedMedia{key: key, displayKey: displayKey, contentType: "image/heic", size: size, width: bounds.Dx(), height: bounds.Dy()}, nil
}
