package tmux

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSplitPane(t *testing.T) {
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
			err := splitPane(t.Context(), mock, "%42", "/work/my branch", tc.command)
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
	t.Run("format cwd", func(t *testing.T) {
		mock := &mockWorkspaceTmux{}
		require.ErrorContains(t, splitPane(t.Context(), mock, "%42", "/work/#{pane_id}", ""), "format syntax")
		require.Empty(t, mock.calls)
	})
}
