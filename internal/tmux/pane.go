package tmux

import (
	"context"
	"fmt"
	"strings"
)

// SplitPane places a shell or command beside the caller without changing the
// caller's session options or establishing a kwt workspace session.
func (t *TmuxCommand) SplitPane(ctx context.Context, pane, worktreeDir, command string) error {
	return splitPane(ctx, t, pane, worktreeDir, command)
}

type paneTmux interface {
	RunCommandContext(context.Context, ...string) error
	RunCommandOutputContext(context.Context, ...string) (string, error)
}

func splitPane(ctx context.Context, t paneTmux, pane, worktreeDir, command string) error {
	if err := ValidateStartDirectory(worktreeDir); err != nil {
		return err
	}
	args := []string{"split-window", "-t", pane, "-c", worktreeDir, "--"}
	if command == "" {
		// Read the effective session option, including an inherited global value.
		// An explicit login shell avoids default-command; an empty command would
		// instead create an empty pane on newer tmux versions.
		shell, err := t.RunCommandOutputContext(ctx, "display-message", "-p", "-t", pane, "#{default-shell}")
		if err != nil {
			return fmt.Errorf("read tmux default-shell: %w", err)
		}
		shell = strings.TrimSpace(shell)
		if shell == "" {
			return fmt.Errorf("tmux returned an empty default-shell")
		}
		args = append(args, shell, "-l")
	} else {
		args = append(args, command)
	}
	if err := t.RunCommandContext(ctx, args...); err != nil {
		return fmt.Errorf("split tmux pane: %w", err)
	}
	return nil
}
