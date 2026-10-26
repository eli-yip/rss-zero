package file

import "testing"

func TestHEICObjectContentType(t *testing.T) {
	service := &FileServiceMinio{}
	for key, want := range map[string]string{"chat/original.heic": "image/heic", "chat/original.display.jpg": "image/jpeg"} {
		if got := service.getContentType(key); got != want {
			t.Errorf("%s: got %s, want %s", key, got, want)
		}
	}
}
