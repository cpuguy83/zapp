package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"io"
	"os"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/opencontainers/go-digest"
)

func FromFile(p string) (digest.Digest, []byte, error) {
	f, err := os.Open(p)
	if err != nil {
		return "", nil, err
	}
	return FromReader(f)
}

func FromReader(r io.Reader) (digest.Digest, []byte, error) {
	h := sha256.New()
	buf := bytes.NewBuffer(nil)
	r = io.TeeReader(io.TeeReader(r, buf), h)

	return digest.FromBytes(h.Sum(nil)), buf.Bytes(), json.NewDecoder(r).Decode(&v1.Manifest{})
}
