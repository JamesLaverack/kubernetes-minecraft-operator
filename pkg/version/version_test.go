package version

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseMinorVersion(t *testing.T) {
	t.Run("minor only", func(t *testing.T) {
		assert.Equal(t, "1.19", ParseMinorVersion("1.19"))
	})
	t.Run("patch", func(t *testing.T) {
		assert.Equal(t, "1.19", ParseMinorVersion("1.19.1"))
	})
}
