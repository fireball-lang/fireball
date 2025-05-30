package lsp

import (
	"go.lsp.dev/uri"
	"os"
)

type document struct {
	uri  uri.URI
	path string

	contents  string
	forceRead bool
	changed   bool

	version int32

	hasPublishedDiagnostics bool
}

func newDocument(path string) *document {
	return &document{
		uri:       uri.File(path),
		path:      path,
		contents:  "",
		forceRead: true,
		changed:   false,
	}
}

func (d *document) AbsolutePath() string {
	return d.path
}

func (d *document) Contents() (string, bool) {
	if d.forceRead {
		b, err := os.ReadFile(d.path)
		if err == nil {
			d.contents = string(b)
			d.changed = true
		}

		d.forceRead = false
	}

	if !d.changed {
		return "", false
	}

	contents := d.contents

	d.contents = ""
	d.changed = false

	return contents, true
}
