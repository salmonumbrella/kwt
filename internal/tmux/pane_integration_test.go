package tmux

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.kenn.io/kwt/internal/utils"
)

func TestSplitPaneOnCallerServer(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not installed; current-server pane behavior not exercised")
	}
	socket := fmt.Sprintf("kwt-pane-%d-%d", os.Getpid(), time.Now().UnixNano())
	t.Cleanup(func() { killPrivateTmuxServer(socket) })
	run := func(args ...string) string {
		t.Helper()
		out, err := runTmuxSocket(t, socket, nil, args...)
		require.NoError(t, err, out)
		return strings.TrimSpace(out)
	}
	caller := strings.Split(run("new-session", "-d", "-P", "-F", "#{socket_path}|#{pid}|#{pane_id}|#{window_id}", "-s", "caller", "-x", "120", "-y", "60"), "|")
	require.Len(t, caller, 4)
	t.Setenv("TMUX", caller[0]+","+caller[1]+",0")
	t.Setenv("TMUX_PANE", caller[2])
	run("set-option", "-t", "caller", "default-shell", "/bin/sh")
	run("set-option", "-t", "caller", "default-command", "exit 99")
	dir := t.TempDir()
	command := NewTmuxCommand("tmux") // No -L: the caller's socket must win.
	require.NoError(t, command.SplitPane(t.Context(), dir, ""))
	pane := run("display-message", "-p", "-t", "caller", "#{pane_id}")
	require.NotEqual(t, caller[2], pane)
	require.Regexp(t, `^[1-9][0-9]*$`, run("display-message", "-p", "-t", pane, "#{pane_pid}"))
	require.Equal(t, caller[3], run("display-message", "-p", "-t", pane, "#{window_id}"))
	require.Equal(t, utils.CanonicalPath(dir), utils.CanonicalPath(run("display-message", "-p", "-t", pane, "#{pane_current_path}")))
	require.Eventually(t, func() bool {
		return run("display-message", "-p", "-t", pane, "#{pane_dead}|#{pane_current_command}") == "0|sh"
	}, time.Second, 10*time.Millisecond, "plain shell must run despite default-command")
	marker := filepath.Join(dir, "agent marker")
	require.NoError(t, command.SplitPane(t.Context(), dir, "printf agent > \""+utils.EscapeForShell(marker)+"\"; exec /bin/sh"))
	require.Eventually(t, func() bool { data, err := os.ReadFile(marker); return err == nil && string(data) == "agent" }, time.Second, 10*time.Millisecond)
	require.Equal(t, "caller", run("list-sessions", "-F", "#{session_name}"))
	require.Equal(t, "exit 99", run("show-options", "-v", "-t", "caller", "default-command"))
	require.Len(t, strings.Split(run("list-panes", "-t", caller[3], "-F", "#{pane_id}"), "\n"), 3)
}
