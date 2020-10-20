package main

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"github.com/opencontainers/go-digest"
	"golang.org/x/crypto/ssh/terminal"
)

func main() {
	if len(os.Args) < 3 {
		errOut(errors.New("usage: " + filepath.Base(os.Args[0]+" <repo> <file> [<media type>]")))
	}

	var mt string
	if len(os.Args) == 4 {
		mt = os.Args[3]
	}

	f, desc, err := FromFile(os.Args[2], mt)
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

	ref := os.Args[1]
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

	_, err = io.Copy(w, rdr)
	if err != nil {
		errOut(err)
	}

	dgst := digest.FromBytes(h.Sum(nil))
	if err := w.Commit(ctx, desc.Size, dgst); err != nil {
		errOut(err)
	}

	fmt.Println("Type:", desc.MediaType)
	fmt.Println("Size:", desc.Size)
	fmt.Println("Digest:", dgst.String())
}
