package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/remotes"
	"github.com/containerd/containerd/remotes/docker"
	"github.com/cpuguy83/dockercfg"
	"github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
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

	resolver := &resolverWrapper{}
	resolver.Resolver = getResolver(resolver.authorize)

	if fileName == "" {
		// no file or sha is given, assume this is just a manifest request
		if err := fetch(ctx, resolver, ref, "", mt); err != nil {
			errOut(err)
		}
		return
	}

	f, desc, err := FromFile(fileName, mt)
	var dgst digest.Digest
	switch {
	case err == nil:
		defer f.Close()
		// Just always ask for creds on push
		resolver.err = docker.ErrInvalidAuthorization
		if err := push(ctx, resolver, ref, desc, f); err != nil {
			errOut(err)
		}
		return
	case os.IsNotExist(err):
		if strings.Contains(fileName, "application/vnd.") {
			// This is probably a mediatype
			mt = fileName
		} else {
			// Maybe this is a digest, in which case the user is requesting to fetch this specifiy digest from the repository
			var err2 error
			dgst, err2 = digest.Parse(fileName)
			if err2 != nil {
				errOut(err)
			}
		}
		if err := fetch(ctx, resolver, ref, dgst, mt); err != nil {
			errOut(err)
		}
		return
	}
}

func fetch(ctx context.Context, resolver *resolverWrapper, ref string, dgst digest.Digest, mt string) (retErr error) {
	defer func() {
		if retErr != nil {
			retErr = fmt.Errorf("fetch: %w", retErr)
		}
	}()
	fetcher, err := resolver.Fetcher(ctx, ref)
	if err != nil {
		return err
	}

	if mt == "application/vnd.docker.plugin.v1+json" {
		scope, err := pluginScope(ref)
		if err != nil {
			return err
		}
		ctx = docker.WithScope(ctx, scope)
	}

	_, desc, err := resolver.Resolve(ctx, ref)
	if err != nil {
		if errors.Is(err, docker.ErrInvalidAuthorization) {
			if !resolver.haveCreds() {
				log.G(ctx).WithError(err).Debug("forcing authorizer to send registry creds")
				resolver.err = err
				_, desc, err = resolver.Resolve(ctx, ref)
			}

			if errors.Is(err, docker.ErrInvalidAuthorization) {
				var scope string
				scope, err = pluginScope(ref)
				if err != nil {
					return err
				}
				log.G(ctx).WithField("scope", scope).WithError(err).Debug("Attemping again with plugin scope")
				ctx = docker.WithScope(ctx, scope)
				_, desc, err = resolver.Resolve(ctx, ref)
			}
		}

		if err != nil {
			return fmt.Errorf("error resolving reference: %w", err)
		}
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

func push(ctx context.Context, resolver *resolverWrapper, ref string, desc v1.Descriptor, f io.Reader) (retErr error) {
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
		return fmt.Errorf("error getting pusher: %w", err)
	}

	w, err := pusher.Push(ctx, desc)
	if err != nil {
		if errdefs.IsAlreadyExists(err) {
			return nil
		}

		scope, err := pluginScope(ref)
		if err != nil {
			return err
		}
		ctx = docker.WithScope(ctx, scope)
		w, err = pusher.Push(ctx, desc)
		if err != nil {
			if errdefs.IsAlreadyExists(err) {
				return nil
			}
			return fmt.Errorf("error starting push: %w", err)
		}
	}

	buf := make([]byte, 1<<20)
	if _, err := io.CopyBuffer(w, f, buf); err != nil {
		return fmt.Errorf("error copying to remote: %w", err)
	}

	if err := w.Commit(ctx, desc.Size, desc.Digest); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

type resolverWrapper struct {
	remotes.Resolver
	u, p string
	err  error
}

func (r *resolverWrapper) authorize(host string) (string, string, error) {
	ctx := context.Background()
	l := log.G(ctx).WithField("host", host)

	if r.haveCreds() {
		l.Debugf("Using cached credentials")
		return r.u, r.p, nil
	}

	host = dockercfg.ResolveRegistryHost(host)
	l = l.WithField("host.resolved", host)

	u, p, err := dockercfg.GetRegistryCredentials(host)
	if err != nil && r.err != nil {
		return "", "", err
	}

	if p != "" {
		r.u = u
		r.p = p
		return u, p, nil
	}

	if !errors.Is(r.err, docker.ErrInvalidAuthorization) {
		// We don't have creds, so...
		// We could ask the terminal for creds, but DockerHub likes to ask for creds for all requests...
		// which we may need to do anyway due to throttling but for now let's just return early and not
		// bug the user since the request may go through without them.
		l.Debug("No creds available, but this is the first round, so we won't fallback to asking the user for them on the terminal")
		return "", "", nil
	}

	if !terminal.IsTerminal(int(os.Stdin.Fd())) {
		// No other way to get creds here
		l.Debug("No terminal deteched, we really won't be getting any creds here")
		return "", "", nil
	}

	r.err = nil
	l.Debug("Getting terminal creds")
	r.u, r.p, err = terminalCreds(host)
	return r.u, r.p, err
}

func (r *resolverWrapper) haveCreds() bool {
	return r.p != ""
}

func pluginScope(ref string) (string, error) {
	u, err := url.Parse("dummy://" + ref)
	if err != nil {
		return "", fmt.Errorf("could not pasre ref %s: %w", ref, err)
	}

	p := strings.SplitN(u.Path, ":", 2)[0]
	return "repository(plugin):" + strings.TrimPrefix(p, "/") + ":pull,push", nil
}
