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

	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/remotes"
	"github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
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

	if len(flag.Args()) < 1 {
		errOut(errors.New("usage: " + filepath.Base(os.Args[0]+" <repo> <file> [<media type>]")))
	}

	ref := flag.Arg(0)
	fileName := flag.Arg(1)
	mt := flag.Arg(2)

	var credsFunc func(string) (string, string, error)
	if terminal.IsTerminal(syscall.Stdin) {
		credsFunc = terminalCreds
	}

	resolver := getResolver(credsFunc)

	if fileName == "" {
		// no file or sha is given, assume this is just a manifest request
		if err := fetch(ctx, resolver, ref, "", mt); err != nil {
			errOut(err)
		}
		return
	}

	f, desc, err := FromFile(fileName, mt)
	switch {
	case err == nil:
		defer f.Close()
		if err := push(ctx, resolver, ref, desc, f); err != nil {
			errOut(err)
		}
		return
	case os.IsNotExist(err):
		dgst, err2 := digest.Parse(fileName)
		if err2 != nil {
			errOut(err)
		}
		if err := fetch(ctx, resolver, ref, dgst, mt); err != nil {
			errOut(err)
		}
		return
	}
}

func fetch(ctx context.Context, resolver remotes.Resolver, ref string, dgst digest.Digest, mt string) (retErr error) {
	defer func() {
		if retErr != nil {
			retErr = fmt.Errorf("fetch: %w", retErr)
		}
	}()
	fetcher, err := resolver.Fetcher(ctx, ref)
	if err != nil {
		return err
	}

	_, desc, err := resolver.Resolve(ctx, ref)
	if err != nil {
		return fmt.Errorf("error resolving reference: %w", err)
	}

	if dgst != "" {
		desc.Digest = dgst
		desc.MediaType = mt
	}

	for i := 0; i < 2; i++ {
		rdr, err := fetcher.Fetch(ctx, desc)
		if err != nil {
			return fmt.Errorf("error fetching content: %w", err)
		}
		defer rdr.Close()

		h := desc.Digest.Algorithm().Digester().Hash()
		r := io.TeeReader(rdr, h)

		buf := make([]byte, 1<<20)

		_, err = io.CopyBuffer(os.Stdout, r, buf)
		if err != nil {
			if errdefs.IsNotFound(err) && mt == "" {
				desc.MediaType = v1.MediaTypeImageManifest
				continue
			}
			return fmt.Errorf("error reading content: %w", err)
		}

		if err := rdr.Close(); err != nil {
			return err
		}

		if desc.Digest != digest.NewDigest(digest.Canonical, h) {
			return errors.New("digest mistmatch")
		}
		return
	}
	return nil
}

func push(ctx context.Context, resolver remotes.Resolver, ref string, desc v1.Descriptor, f io.Reader) (retErr error) {
	defer func() {
		if retErr != nil {
			retErr = fmt.Errorf("push: %w", retErr)
		}
	}()

	fmt.Println("Type:", desc.MediaType)
	fmt.Println("Size:", desc.Size)
	fmt.Println("Digest:", desc.Digest)

	pusher, err := resolver.Pusher(ctx, ref)
	if err != nil {
		errOut(err)
	}

	w, err := pusher.Push(ctx, desc)
	if err != nil {
		if errdefs.IsAlreadyExists(err) {
			return nil
		}
		return err
	}

	buf := make([]byte, 1<<20)
	if _, err := io.CopyBuffer(w, f, buf); err != nil {
		return fmt.Errorf("error copying to remote: %w", err)
	}

	if err := w.Commit(ctx, desc.Size, desc.Digest); err != nil {
		return err
	}
	return nil
}
