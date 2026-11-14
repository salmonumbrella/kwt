package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"go.kenn.io/kwt/internal/discovery"
	"go.kenn.io/kwt/internal/pullrequest"
	"go.kenn.io/kwt/internal/url"
	"go.kenn.io/kwt/pkg/models"
)

func setOpenPaneTestFlags(t *testing.T, flags ...string) *cobra.Command {
	t.Helper()
	for _, name := range []string{"pane", "agent"} {
		flag := openCmd.Flags().Lookup(name)
		require.NotNil(t, flag)
		old, changed := flag.Value.String(), flag.Changed
		t.Cleanup(func() { require.NoError(t, flag.Value.Set(old)); flag.Changed = changed })
	}
	require.NoError(t, openCmd.Flags().Set("pane", "true"))
	require.NoError(t, openCmd.Flags().Set("agent", ""))
	openCmd.Flags().Lookup("agent").Changed = false
	cmd := &cobra.Command{}
	cmd.SetContext(t.Context())
	markCommandFlagsChanged(t, cmd, flags...)
	t.Setenv("TMUX", "/tmp/caller,123,0")
	t.Setenv("TMUX_PANE", "%42")
	return cmd
}

func TestOpenPaneRejectsIncompatibleFlags(t *testing.T) {
	for _, flag := range []string{"layout", "select-layout", "start-session", "json", "expected-repository", "expected-registration", "expected-generation", "expected-session"} {
		t.Run(flag, func(t *testing.T) {
			cmd := setOpenPaneTestFlags(t, flag)
			err := runOpenWithContext(cmd, nil, &CommandContext{Config: &models.Config{}})
			require.ErrorContains(t, err, "--pane")
			require.ErrorContains(t, err, "--"+flag)
		})
	}
	t.Run("agent requires pane", func(t *testing.T) {
		cmd := setOpenPaneTestFlags(t, "agent")
		openPane = false
		err := runOpenWithContext(cmd, nil, &CommandContext{Config: &models.Config{}})
		require.ErrorContains(t, err, "--agent requires --pane")
	})
	t.Run("outside tmux", func(t *testing.T) {
		cmd := setOpenPaneTestFlags(t)
		t.Setenv("TMUX", "")
		err := runOpenWithContext(cmd, nil, &CommandContext{Config: &models.Config{}})
		require.ErrorContains(t, err, "inside tmux")
	})
	t.Run("directory workspace", func(t *testing.T) {
		cmd := setOpenPaneTestFlags(t)
		path := t.TempDir()
		err := runOpenWithContext(cmd, []string{path}, &CommandContext{Config: &models.Config{Workspaces: []models.Workspace{{Path: path}}}})
		require.ErrorContains(t, err, "--pane supports Git worktrees only")
	})
}

func TestOpenPaneSelectedWorktree(t *testing.T) {
	for _, tc := range []struct{ name, agent, command, want string }{
		{name: "shell"}, {name: "agent", agent: "codex", command: "codex --model 'my model'"},
		{name: "unknown", agent: "missing", want: "unknown agent"},
		{name: "empty command", agent: "empty", want: "empty command"},
		{name: "explicit empty agent", want: "nonempty"},
		{name: "protected", want: "kwt pr attach"},
		{name: "stale generation", want: "generation"},
		{name: "split failure", want: "cannot split"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := setOpenPaneTestFlags(t)
			openAgent = tc.agent
			if tc.name == "explicit empty agent" {
				markCommandFlagsChanged(t, cmd, "agent")
			}
			initCommandTestConfig(t, t.TempDir())
			path := newTUITestRepo(t)
			generation := tuiTestWorktreeGeneration(t, path, path)
			require.NoError(t, os.WriteFile(filepath.Join(path, ".kwt.toml"), []byte("invalid = ["), 0o600))
			entry := &discovery.GlobalWorktreeEntry{Path: path, Generation: generation, RepositoryInfo: &url.RepositoryInfo{FullPath: "github.com/acme/widget"}}
			if tc.name == "stale generation" {
				entry.Generation = "0123456789abcdef0123456789abcdef"
			}
			if tc.name == "protected" {
				require.NoError(t, pullrequest.NewFileStore(prStorePath()).Update(t.Context(), func(records map[string]pullrequest.Provenance) error {
					records["pr-32"] = pullrequest.Provenance{Workspace: pullrequest.Workspace{Path: path, Generation: generation}}
					return nil
				}))
			}
			oldDiscover, oldSplit, oldRunner := discoverOpenWorktree, splitOpenPane, newOpenWorkspaceRunner
			t.Cleanup(func() {
				discoverOpenWorktree = oldDiscover
				splitOpenPane = oldSplit
				newOpenWorkspaceRunner = oldRunner
			})
			discoverOpenWorktree = func(string, []models.Project) (*discovery.GlobalWorktreeEntry, error) { return entry, nil }
			calls := 0
			splitOpenPane = func(ctx context.Context, names []string, dir, command string) error {
				calls++
				require.Equal(t, t.Context(), ctx)
				require.Equal(t, path, dir)
				require.Equal(t, tc.command, command)
				require.Contains(t, names, "CUSTOM_FLEET_TOKEN")
				if tc.name == "split failure" {
					return fmt.Errorf("cannot split")
				}
				return nil
			}
			newOpenWorkspaceRunner = func([]string) openWorkspaceRunner {
				t.Fatal("pane mode must not establish or attach a workspace session")
				return nil
			}
			cfg := &models.Config{Agents: map[string]string{"codex": "codex --model 'my model'", "empty": " "}, Fleet: models.FleetConfig{TokenEnv: "CUSTOM_FLEET_TOKEN"}, Layouts: models.LayoutsConfig{Default: "missing"}}
			err := runOpenWithContext(cmd, []string{path}, &CommandContext{Config: cfg})
			if tc.want != "" {
				require.ErrorContains(t, err, tc.want)
			} else {
				require.NoError(t, err)
			}
			if tc.want == "" || tc.name == "split failure" {
				require.Equal(t, 1, calls)
			} else {
				require.Zero(t, calls)
			}
		})
	}
}
