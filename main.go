package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"syscall"

	"github.com/containerd/containerd/remotes/docker"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"golang.org/x/crypto/ssh/terminal"
)

func main() {
	if len(os.Args) < 3 {
		errOut(errors.New("usage: " + filepath.Base(os.Args[0]+" <repo> <manifest.json>")))
	}
	dgst, data, err := FromFile(os.Args[2])
	if err != nil {
		errOut(err)
	}

	var credsFunc func(string) (string, string, error)
	if terminal.IsTerminal(syscall.Stdin) {
		credsFunc = terminalCreds
	}

	resolver := docker.NewResolver(docker.ResolverOptions{
		Hosts: docker.ConfigureDefaultRegistries(
			docker.WithAuthorizer(docker.NewDockerAuthorizer(
				docker.WithAuthCreds(credsFunc),
			)),
		),
	})

	ctx := context.Background()

	ref := os.Args[1]
	pusher, err := resolver.Pusher(ctx, ref)
	if err != nil {
		errOut(err)
	}

	desc := v1.Descriptor{
		MediaType: v1.MediaTypeImageManifest,
		Digest:    dgst,
		Size:      int64(len(data)),
	}

	w, err := pusher.Push(ctx, desc)
	if _, err := io.Copy(w, bytes.NewReader(data)); err != nil {
		errOut(err)
	}
}

func errOut(err error) {
	if err == nil {
		return
	}

	_, f, l, _ := runtime.Caller(1)
	if wd, err := os.Getwd(); err == nil {
		if rel, err := filepath.Rel(filepath.Dir(f), wd); err == nil {
			f = filepath.Join(rel, filepath.Base(f))
		}
	}
	fmt.Fprintf(os.Stderr, "%s:%d %s\n", f, l, err.Error())
	os.Exit(1)
}

func terminalCreds(host string) (string, string, error) {
	fmt.Fprintf(os.Stderr, "Username: ")

	stdin := bufio.NewReader(os.Stdin)
	user, _, err := stdin.ReadLine()
	if err != nil {
		return "", "", err
	}

	fmt.Fprintf(os.Stderr, "Password: ")
	pass, err := terminal.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return "", "", err
	}

	return string(user), string(pass), nil
}
