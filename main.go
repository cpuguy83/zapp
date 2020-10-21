package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"github.com/containerd/containerd/log"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh/terminal"
)

var (
	allowHTTP bool
	debug     bool
)

func main() {
	flag.BoolVar(&allowHTTP, "allow-http", false, "allow fallback to http")
	flag.BoolVar(&debug, "debug", false, "allow fallback to http")
	flag.Parse()

	ctx := context.Background()

	if debug {
		l := logrus.NewEntry(logrus.StandardLogger())
		l.Logger.SetOutput(os.Stderr)
		l.Logger.SetLevel(logrus.DebugLevel)
		ctx = log.WithLogger(ctx, l)
	}

	if len(flag.Args()) < 2 {
		errOut(errors.New("usage: " + filepath.Base(os.Args[0]+" <repo> <file> [<media type>]")))
	}

	ref := flag.Arg(0)
	fileName := flag.Arg(1)
	mt := flag.Arg(2)

	f, desc, err := FromFile(fileName, mt)
	if err != nil {
		errOut(err)
	}
	defer f.Close()

	var credsFunc func(string) (string, string, error)
	if terminal.IsTerminal(syscall.Stdin) {
		credsFunc = terminalCreds
	}

	resolver := getResolver(credsFunc)

	fmt.Println("Type:", desc.MediaType)
	fmt.Println("Size:", desc.Size)
	fmt.Println("Digest:", desc.Digest)

	pusher, err := resolver.Pusher(ctx, ref)
	if err != nil {
		errOut(err)
	}

	w, err := pusher.Push(ctx, desc)
	if err != nil {
		errOut(err)
	}

	buf := make([]byte, 1<<20)
	if _, err := io.CopyBuffer(w, f, buf); err != nil {
		errOut(fmt.Errorf("error copying to remote: %w", err))
	}

	if err := w.Commit(ctx, desc.Size, desc.Digest); err != nil {
		errOut(err)
	}
}
