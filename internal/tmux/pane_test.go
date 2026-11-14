package tmux

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSplitPane(t *testing.T) {
	t.Setenv("TMUX", "/tmp/caller server,123,0")
	t.Setenv("TMUX_PANE", "%42")
	for _, tc := range []struct {
		name, command, shell, want string
		outputErr                  error
		failRun                    int
	}{
		{name: "shell", shell: "/bin/zsh"},
		{name: "agent", command: "codex --model 'my model'; echo done"},
		{name: "shell query failure", outputErr: errors.New("query failed"), want: "query failed"},
		{name: "empty shell", want: "empty default-shell"},
		{name: "split failure", command: "codex", failRun: 1, want: "split tmux pane"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mock := &mockWorkspaceTmux{sessionDefaultShell: tc.shell, outputErr: tc.outputErr, failRunOnCall: tc.failRun}
			err := splitPane(t.Context(), mock, "/work/my branch", tc.command)
			if tc.want != "" {
				require.ErrorContains(t, err, tc.want)
				return
			}
			require.NoError(t, err)
			split := []string{"split-window", "-t", "%42", "-c", "/work/my branch", "--"}
			var want [][]string
			if tc.command == "" {
				want = append(want, []string{"display-message", "-p", "-t", "%42", "#{default-shell}"})
				split = append(split, tc.shell, "-l")
			} else {
				split = append(split, tc.command)
			}
			want = append(want, split)
			require.Equal(t, want, mock.calls, "placement must not create, attach, bootstrap, or arrange a session")
		})
	}
	for _, tc := range []struct{ name, tmux, pane, path, want string }{
		{"outside", "", "", "/work", "inside tmux"},
		{"missing caller pane", "/tmp/server,123,0", "", "/work", "inside tmux"},
		{"format cwd", "/tmp/server,123,0", "%42", "/work/#{pane_id}", "format syntax"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("TMUX", tc.tmux)
			t.Setenv("TMUX_PANE", tc.pane)
			mock := &mockWorkspaceTmux{}
			require.ErrorContains(t, splitPane(t.Context(), mock, tc.path, ""), tc.want)
			require.Empty(t, mock.calls)
		})
	}
}
