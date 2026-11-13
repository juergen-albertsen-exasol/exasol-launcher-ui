package opener

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCommandPerPlatform(t *testing.T) {
	t.Parallel()

	for goos, expected := range map[string]string{
		"darwin":  "open",
		"linux":   "xdg-open",
		"windows": "explorer",
	} {
		name, args := command(goos, "/deployments/default")

		assert.Equal(t, expected, name, goos)
		assert.Equal(t, []string{"/deployments/default"}, args, goos)
	}
}
