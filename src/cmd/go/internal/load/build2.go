package load

import (
	"bytes"
	"strings"
)

var (
	slashSlash  = []byte("//")
	bSlashSlash = []byte(slashSlash)
)

func builds(content []byte) (_ []string, err error) {
	// Pass 1. Identify leading run of // comments and blank lines,
	// which must be followed by a blank line.
	// Also identify any //go:build comments.
	content, _, _, err = build.ParseFileHeader(content)
	if err != nil {
		return nil, err
	}

	// Pass 2.  Process each +build line in the run.
	p := content
	var builds []string
	for len(p) > 0 {
		line := p
		if i := bytes.IndexByte(line, '\n'); i >= 0 {
			line, p = line[:i], p[i+1:]
		} else {
			p = p[len(p):]
		}
		line = bytes.TrimSpace(line)
		if !bytes.HasPrefix(line, bSlashSlash) {
			continue
		}
		line = bytes.TrimSpace(line[len(bSlashSlash):])
		if len(line) > 0 && line[0] == '+' {
			// Looks like a comment +line.
			f := strings.Fields(string(line))
			if f[0] == "+build" {
				for _, tok := range f[1:] {
					builds = append(builds, tok)
				}
			}
		}
	}
	return builds, nil
}
