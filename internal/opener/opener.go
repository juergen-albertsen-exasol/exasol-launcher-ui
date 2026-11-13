package opener

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
)

// Open opens target, a URL or a folder, with the host's default application.
func Open(ctx context.Context, target string) error {
	name, args := command(runtime.GOOS, target)

	err := exec.CommandContext(ctx, name, args...).Run()

	// Explorer exits with status 1 even when it opened the target.
	if _, isExit := errors.AsType[*exec.ExitError](err); isExit && runtime.GOOS == "windows" {
		return nil
	}

	if err != nil {
		return fmt.Errorf("cannot open %s with %s: %w", target, name, err)
	}

	return nil
}

func command(goos, target string) (string, []string) {
	switch goos {
	case "darwin":
		return "open", []string{target}
	case "windows":
		return "explorer", []string{target}
	default:
		return "xdg-open", []string{target}
	}
}
