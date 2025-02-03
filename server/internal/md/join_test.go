package md

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestJoin(t *testing.T) {
	t.Run("No strings", func(t *testing.T) {
		assert.Equal(t, "", Join())
	})

	t.Run("Single string", func(t *testing.T) {
		assert.Equal(t, "hello\n\n", Join("hello"))
	})

	t.Run("Multiple strings", func(t *testing.T) {
		assert.Equal(t, "hello\n\nworld\n\n", Join("hello", "world"))
	})

	t.Run("Empty string included", func(t *testing.T) {
		assert.Equal(t, "hello\n\nworld\n\n", Join("hello", "", "world"))
	})

	t.Run("Strings with special characters", func(t *testing.T) {
		assert.Equal(t, "hello\n\nworld\n\nline\n1\n\nline\t2\n\n", Join("hello", "world", "line\n1", "line\t2"))
	})
}
