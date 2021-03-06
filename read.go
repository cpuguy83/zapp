package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

func FromFile(p string, mt string) (io.ReadCloser, v1.Descriptor, error) {
	var desc v1.Descriptor

	stat, err := os.Stat(p)
	if err != nil {
		return nil, desc, err
	}

	if stat.IsDir() {
		// TODO: detech OCI image layout or fallback to tar+gz as a layer blob
		return nil, desc, errors.New("dir is not supported")
	}

	desc.MediaType = mt
	desc.Size = stat.Size()
	if desc.Size == 0 {
		return nil, desc, errors.New("empty file")
	}

	f, err := os.OpenFile(p, os.O_RDONLY, 0)
	if err != nil {
		return nil, desc, err
	}

	if desc.MediaType == "" {
		buffered := bufio.NewReader(f)
		header, _ := buffered.Peek(10)

		switch {
		case DetectCompression(header) == Gzip:
			desc.MediaType = v1.MediaTypeImageLayerGzip
		default:
			var mt mediaType
			// ignore errors here, we'll error below if the media type is no good
			json.NewDecoder(buffered).Decode(&mt)
			desc.MediaType = mt.MediaType
		}
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return nil, desc, fmt.Errorf("error seeking to file start after deteching media type: %w", err)
		}
	}

	if desc.MediaType == "" {
		return nil, desc, errors.New("could not determine media type")
	}

	h := digest.Canonical.Digester().Hash()
	io.Copy(ioutil.Discard, io.TeeReader(f, h))

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, desc, fmt.Errorf("error seeking to file start after hashing: %w", err)
	}

	desc.Digest = digest.NewDigest(digest.Canonical, h)

	return f, desc, err
}

type mediaType struct {
	MediaType string `json:"mediaType"`
}

type Compression int

const (
	Uncompressed Compression = 0
	Gzip         Compression = 1
)

func DetectCompression(source []byte) Compression {
	for compression, m := range map[Compression][]byte{
		Gzip: {0x1F, 0x8B, 0x08},
	} {
		if len(source) < len(m) {
			// Len too short
			continue
		}
		if bytes.Equal(m, source[:len(m)]) {
			return compression
		}
	}
	return Uncompressed
}
