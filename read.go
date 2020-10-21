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
		header, err := buffered.Peek(10)
		if err != nil {
			// Note: Technically this could be that the file isn't even 10B and it could be valid(?)
			// But this is very unlikely, so just error out
			return nil, desc, err
		}

		switch {
		case DetectCompression(header) == Gzip:
			desc.MediaType = v1.MediaTypeImageLayerGzip
		default:
			var mt mediaType
			if err := json.NewDecoder(buffered).Decode(&mt); err != nil || mt.MediaType == "" {
				return nil, desc, fmt.Errorf("could not determine media type: %w", err)
			}

			desc.MediaType = mt.MediaType
		}
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return nil, desc, fmt.Errorf("error seeking to file start after deteching media type: %w", err)
		}
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
