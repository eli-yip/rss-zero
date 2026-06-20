package migrate

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type fakeNotifier struct {
	count int
	last  string
}

func (f *fakeNotifier) Notify(_, content string) error {
	f.count++
	f.last = content
	return nil
}

func TestRunWithNotifiesOnFailure(t *testing.T) {
	fn := &fakeNotifier{}
	run := runWith(nil, zap.NewNop(), fn)
	err := run(Migration{
		Version: 1, Name: "boom",
		Run: func(*gorm.DB, *zap.Logger) error { return errors.New("nope") },
	})
	assert.Error(t, err)
	assert.Equal(t, 1, fn.count)
	assert.Contains(t, fn.last, "boom")
}

func TestRunWithNilNotifierDoesNotPanic(t *testing.T) {
	run := runWith(nil, zap.NewNop(), nil)
	err := run(Migration{
		Version: 1, Name: "x",
		Run: func(*gorm.DB, *zap.Logger) error { return errors.New("e") },
	})
	assert.Error(t, err)
}
