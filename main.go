package main

import (
	"errors"
	"fmt"
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
	opts, err := parseArgs(args, stdout, stderr)
	switch {
	case errors.Is(err, errHelp):
		return 0
	case errors.Is(err, errUsage):
		return 1
	case err != nil:
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	if !opts.skipFetch {
		client, err := newClient()
		if err != nil {
			fmt.Fprintf(stderr, "error: %v (try `gh auth login`)\n", err)
			return 1
		}
		if err := fetchSBOMs(client, opts, stderr); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
	}

	rows, err := aggregate(opts)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	summarize(rows, opts, stdout)
	return 0
}
