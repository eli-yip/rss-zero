package weibochat

import (
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"

	_ "golang.org/x/image/webp"
)

// ReadImageDimensions reads only the image header, without decoding its pixels.
// The bound also limits buffering when replaying the header into object storage.
func ReadImageDimensions(r io.Reader) (width, height int, err error) {
	config, _, err := image.DecodeConfig(io.LimitReader(r, 8<<20))
	if err != nil {
		return 0, 0, err
	}
	return config.Width, config.Height, nil
}
