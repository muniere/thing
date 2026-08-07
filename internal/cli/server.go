package cli

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/muniere/thing/internal/daemon"
	"github.com/muniere/thing/internal/registry"
)

// ExitError makes a command exit with a specific status code without the usual
// "thing: <err>" message; the command has already written its own output. It is
// how `server status` reports "stopped" on stdout yet exits non-zero.
type ExitError struct{ Code int }

func (e *ExitError) Error() string { return fmt.Sprintf("exit status %d", e.Code) }

// newServerCmd builds the `thing server` group: control for the thingd web
// daemon. Unlike the tree commands it operates on the single global daemon and
// ignores the project data directory, using only the state directory.
func newServerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Control the thingd web server daemon",
	}
	cmd.AddCommand(
		newServerStartCmd(),
		newServerStopCmd(),
		newServerRestartCmd(),
		newServerStatusCmd(),
		newServerLogsCmd(),
	)
	return cmd
}

func newServerStartCmd() *cobra.Command {
	var port int
	var open bool
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the thingd daemon in the background",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := daemon.Start(daemon.StartOptions{Port: port})
			if errors.Is(err, daemon.ErrAlreadyRunning) {
				if cur, running, serr := daemon.Status(); serr == nil && running {
					return fmt.Errorf("already running (pid %d) at %s", cur.PID, cur.URL)
				}
				return err
			}
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "thingd started (pid %d) at %s\n", st.PID, st.URL)
			if open {
				daemon.OpenBrowser(st.URL)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&port, "port", daemon.DefaultPort, "listen port")
	cmd.Flags().BoolVar(&open, "open", false, "open the URL in a browser once started")
	return cmd
}

func newServerStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the running thingd daemon",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			err := daemon.Stop()
			if errors.Is(err, daemon.ErrNotRunning) {
				fmt.Fprintln(cmd.OutOrStdout(), "not running")
				return nil
			}
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "stopped")
			return nil
		},
	}
}

func newServerRestartCmd() *cobra.Command {
	var port int
	cmd := &cobra.Command{
		Use:   "restart",
		Short: "Restart the thingd daemon",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Stopping a not-running daemon is fine: restart doubles as start.
			if err := daemon.Stop(); err != nil && !errors.Is(err, daemon.ErrNotRunning) {
				return err
			}
			st, err := daemon.Start(daemon.StartOptions{Port: port})
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "thingd started (pid %d) at %s\n", st.PID, st.URL)
			return nil
		},
	}
	cmd.Flags().IntVar(&port, "port", daemon.DefaultPort, "listen port")
	return cmd
}

func newServerStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show whether the thingd daemon is running",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			st, running, err := daemon.Status()
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if !running {
				fmt.Fprintln(out, "stopped")
				return &ExitError{Code: 1}
			}
			fmt.Fprintf(out, "running (pid %d, port %d)\n%s\nprojects: %d\n", st.PID, st.Port, st.URL, projectCount())
			return nil
		},
	}
}

// projectCount returns the number of registered projects, or 0 if the registry
// can't be read (status should still report the daemon is up).
func projectCount() int {
	file, err := registry.File()
	if err != nil {
		return 0
	}
	reg, err := registry.Load(file)
	if err != nil {
		return 0
	}
	return len(reg.Projects)
}

func newServerLogsCmd() *cobra.Command {
	var follow bool
	var lines int
	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Show the thingd daemon log",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := daemon.LogPath()
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			tail, ok, err := daemon.ReadLastLines(path, lines)
			if err != nil {
				return err
			}
			if !ok {
				fmt.Fprintln(out, "no log yet")
				if !follow {
					return nil
				}
			}
			for _, line := range tail {
				fmt.Fprintln(out, line)
			}
			if !follow {
				return nil
			}
			// Follow until interrupted (Ctrl-C).
			stop := make(chan struct{})
			sigc := make(chan os.Signal, 1)
			signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)
			go func() { <-sigc; close(stop) }()
			return daemon.Follow(path, out, stop)
		},
	}
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "stream new log lines until interrupted")
	cmd.Flags().IntVarP(&lines, "lines", "n", 0, "show only the last N lines (0 = all)")
	return cmd
}
