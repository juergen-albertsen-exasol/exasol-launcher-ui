package launcher

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// Configuration is the output of `exasol config get --json`.
type Configuration struct {
	Infrastructure Preset `json:"infrastructure"`
	Installation   Preset `json:"installation"`
}

// Preset is one preset of a deployment's configuration.
type Preset struct {
	Identity PresetIdentity `json:"identity"`
	Options  Options        `json:"options"`
}

// PresetIdentity names a preset.
type PresetIdentity struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Description string `json:"description"`
}

// Option is an option's name and its effective value as JSON.
type Option struct {
	Name  string
	Value json.RawMessage
}

// Options keeps a preset's options in the order the launcher reports them.
type Options []Option

func (o *Options) UnmarshalJSON(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))

	if token, err := decoder.Token(); err != nil || token != json.Delim('{') {
		return errors.New("the options are no JSON object")
	}

	options := Options{}

	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return err
		}

		name, isName := token.(string)
		if !isName {
			return errors.New("an option has no name")
		}

		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return err
		}

		options = append(options, Option{Name: name, Value: value})
	}

	*o = options

	return nil
}

// Configuration returns the configuration of the deployment in deploymentDir.
func (c *Client) Configuration(ctx context.Context, deploymentDir string) (Configuration, error) {
	var configuration Configuration

	err := c.runJSON(ctx, &configuration,
		"config", "get", "--json", "--deployment-dir", deploymentDir)

	return configuration, err
}

// Setting is an option value in the form the launcher takes as a flag.
type Setting struct {
	Name  string
	Value string
}

// SetConfiguration sets the given options of the deployment in deploymentDir.
func (c *Client) SetConfiguration(
	ctx context.Context,
	deploymentDir string,
	settings []Setting,
) error {
	args := []string{"config", "set"}
	for _, setting := range settings {
		args = append(args, "--"+setting.Name+"="+setting.Value)
	}

	return c.run(ctx, append(args, "--deployment-dir", deploymentDir)...)
}

// ResetConfiguration resets the named options of the deployment in
// deploymentDir to their preset defaults.
func (c *Client) ResetConfiguration(
	ctx context.Context,
	deploymentDir string,
	names []string,
) error {
	args := append([]string{"config", "reset"}, names...)

	return c.run(ctx, append(args, "--deployment-dir", deploymentDir)...)
}

// LauncherError is a failure the launcher reported itself.
type LauncherError struct {
	Message string
}

func (e *LauncherError) Error() string {
	return e.Message
}

// run runs a short launcher command. When the launcher fails, the error is a
// LauncherError carrying the launcher's error message without its usage text.
func (c *Client) run(ctx context.Context, args ...string) error {
	var stderr bytes.Buffer

	cmd := c.command(ctx, args...)
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		return nil
	}

	if _, isExit := errors.AsType[*exec.ExitError](err); !isExit {
		return fmt.Errorf("cannot run the launcher %q: %w", c.executable, err)
	}

	return &LauncherError{Message: errorMessage(stderr.String(), err)}
}

func errorMessage(stderr string, err error) string {
	last := ""

	for line := range strings.Lines(stderr) {
		line = strings.TrimSpace(line)
		if message, isError := strings.CutPrefix(line, "Error: "); isError {
			return message
		}

		if line != "" {
			last = line
		}
	}

	if last != "" {
		return last
	}

	return err.Error()
}
