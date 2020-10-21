package main

import (
	"context"
	"crypto/sha256"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/log"
	"github.com/opencontainers/go-digest"
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

	pusher, err := resolver.Pusher(ctx, ref)
	if err != nil {
		errOut(err)
	}

	h := sha256.New()
	rdr := io.TeeReader(f, h)

	w, err := pusher.Push(ctx, desc)
	if err != nil {
		errOut(err)
	}

	if err := content.Copy(ctx, w, rdr, desc.Size, desc.Digest); err != nil {
		errOut(err)
	}

	dgst := digest.FromBytes(h.Sum(nil))
	fmt.Println("Type:", desc.MediaType)
	fmt.Println("Size:", desc.Size)
	fmt.Println("Digest:", dgst.String())
}
