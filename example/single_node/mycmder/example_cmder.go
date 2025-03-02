package mycmder

import (
	"io"
	"os"

	"github.com/medyagh/kic/pkg/runner"

	"golang.org/x/crypto/ssh/terminal"
)

// exampale of bringing your own runner.Cmder

const defaultOCI = "docker"

// New creates a new implementor of runner.Cmder
func New(containerNameOrID string) runner.Cmder {
	return &containerCmder{
		nameOrID: containerNameOrID,
	}
}

// containerCmder implements runner.Cmder for docker containers
type containerCmder struct {
	nameOrID string
}

func (c *containerCmder) Command(command string, args ...string) runner.Cmd {
	return &containerCmd{
		nameOrID: c.nameOrID,
		command:  command,
		args:     args,
	}
}

// containerCmd implements runner.Cmd for docker containers
type containerCmd struct {
	nameOrID string // the container name or ID
	command  string
	args     []string
	env      []string
	stdin    io.Reader
	stdout   io.Writer
	stderr   io.Writer
}

func (c *containerCmd) Run() error {
	args := []string{
		"exec",
		// run with privileges so we can remount etc..
		"--privileged",
	}
	if c.stdin != nil {
		args = append(args,
			"-i", // interactive so we can supply input
		)
	}
	// if the command is hooked up to the processes's output we want a tty
	if IsTerminal(c.stderr) || IsTerminal(c.stdout) {
		args = append(args,
			"-t",
		)
	}
	// set env
	for _, env := range c.env {
		args = append(args, "-e", env)
	}
	// specify the container and command, after this everything will be
	// args the the command in the container rather than to docker
	args = append(
		args,
		c.nameOrID, // ... against the container
		c.command,  // with the command specified
	)
	args = append(
		args,
		// finally, with the caller args
		c.args...,
	)
	cmd := runner.Command(defaultOCI, args...)
	if c.stdin != nil {
		cmd.SetStdin(c.stdin)
	}
	if c.stderr != nil {
		cmd.SetStderr(c.stderr)
	}
	if c.stdout != nil {
		cmd.SetStdout(c.stdout)
	}
	return cmd.Run()
}

func (c *containerCmd) SetEnv(env ...string) runner.Cmd {
	c.env = env
	return c
}

func (c *containerCmd) SetStdin(r io.Reader) runner.Cmd {
	c.stdin = r
	return c
}

func (c *containerCmd) SetStdout(w io.Writer) runner.Cmd {
	c.stdout = w
	return c
}

func (c *containerCmd) SetStderr(w io.Writer) runner.Cmd {
	c.stderr = w
	return c
}

// IsTerminal returns true if the writer w is a terminal
func IsTerminal(w io.Writer) bool {
	if v, ok := (w).(*os.File); ok {
		return terminal.IsTerminal(int(v.Fd()))
	}
	return false
}
