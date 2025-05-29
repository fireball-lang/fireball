package lsp

import (
	"go.lsp.dev/uri"
	"os"
)

type document struct {
	uri  uri.URI
	path string

	contents string
	changed  bool

	version int32
}

func newDocument(path string) (*document, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return &document{
		uri:      uri.File(path),
		path:     path,
		contents: string(bytes),
		changed:  true,
	}, nil
}

func (d *document) AbsolutePath() string {
	return d.path
}

func (d *document) Contents() (string, bool) {
	if !d.changed {
		return "", false
	}

	contents := d.contents

	d.contents = ""
	d.changed = false

	return contents, true
}
