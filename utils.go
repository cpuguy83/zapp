package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"

	"github.com/containerd/containerd/remotes"
	"github.com/containerd/containerd/remotes/docker"
	"golang.org/x/crypto/ssh/terminal"
)

func errOut(err error) {
	if err == nil {
		return
	}

	if wd, err2 := os.Getwd(); err2 == nil {
		_, f, l, _ := runtime.Caller(1)
		if rel, err2 := filepath.Rel(filepath.Dir(f), wd); err2 == nil {
			f = filepath.Join(rel, filepath.Base(f))
			fmt.Fprintf(os.Stderr, "%s:%d %s\n", f, l, err.Error())
			os.Exit(1)
		}
	}
	fmt.Fprintln(os.Stderr, err.Error())
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

func getResolver(credsFunc func(string) (string, string, error)) remotes.Resolver {
	return docker.NewResolver(docker.ResolverOptions{
		Hosts: docker.ConfigureDefaultRegistries(
			docker.WithAuthorizer(docker.NewDockerAuthorizer(
				docker.WithAuthCreds(credsFunc),
			)),
		),
	})
}
