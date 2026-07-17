package main

import (
	"io"
	"os"

	"github.com/cli/go-gh/v2/pkg/api"
)

// Swapped out in tests.
var exit = os.Exit

type clientFactory func() (*api.RESTClient, error)

func main() {
	exit(run(os.Args[1:], os.Stdout, os.Stderr, defaultClient))
}

func defaultClient() (*api.RESTClient, error) {
	return api.DefaultRESTClient()
}

func run(args []string, stdout, stderr io.Writer, newClient clientFactory) int {
	cmd := newRootCmd(newClient)
	cmd.SetArgs(args)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	if err := cmd.Execute(); err != nil {
		return 1
	}
	return 0
}
