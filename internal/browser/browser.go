package browser

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
)

// Open opens url in the user's default browser.
func Open(ctx context.Context, url string) error {
	opener := "xdg-open"
	if runtime.GOOS == "darwin" {
		opener = "open"
	}

	if err := exec.CommandContext(ctx, opener, url).Run(); err != nil {
		return fmt.Errorf("cannot open the browser with %s: %w", opener, err)
	}

	return nil
}
