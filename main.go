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

	"github.com/opencontainers/go-digest"
	"golang.org/x/crypto/ssh/terminal"
)

var (
	allowHTTP bool
)

func main() {
	flag.BoolVar(&allowHTTP, "allow-http", false, "allow fallback to http")
	flag.Parse()

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

	ctx := context.Background()

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

	n, err := io.Copy(w, rdr)
	if err != nil {
		errOut(err)
	}

	dgst := digest.FromBytes(h.Sum(nil))
	if err := w.Commit(ctx, n, dgst); err != nil {
		errOut(err)
	}

	fmt.Println("Type:", desc.MediaType)
	fmt.Println("Size:", n)
	fmt.Println("Digest:", dgst.String())
}
