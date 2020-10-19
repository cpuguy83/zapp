package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"syscall"

	"github.com/containerd/containerd/remotes/docker"
	"github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"golang.org/x/crypto/ssh/terminal"
)

func main() {
	var (
		dgst digest.Digest
		data []byte
		err  error
	)

	dgst, data, err = FromFile(os.Args[2])
	if err != nil {
		panic(err)
	}

	resolver := docker.NewResolver(docker.ResolverOptions{
		Hosts: docker.ConfigureDefaultRegistries(
			docker.WithAuthorizer(docker.NewDockerAuthorizer(
				docker.WithAuthCreds(func(host string) (string, string, error) {
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
				}),
			)),
		),
	})

	ctx := context.Background()

	ref := os.Args[1]
	pusher, err := resolver.Pusher(ctx, ref)
	if err != nil {
		panic(err)
	}

	desc := v1.Descriptor{
		MediaType: v1.MediaTypeImageManifest,
		Digest:    dgst,
		Size:      int64(len(data)),
	}
	w, err := pusher.Push(ctx, desc)

	if _, err := io.Copy(w, bytes.NewReader(data)); err != nil {
		panic(err)
	}
}
