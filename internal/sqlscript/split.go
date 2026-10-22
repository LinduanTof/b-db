package sqlscript

import (
	"fmt"
	"strings"
)

func Split(script string) ([]string, error) {
	var out []string
	var b strings.Builder
	var quote byte
	lineComment := false
	blockComment := false

	for i := 0; i < len(script); i++ {
		c := script[i]
		var next byte
		if i+1 < len(script) {
			next = script[i+1]
		}

		if lineComment {
			if c == '\n' {
				lineComment = false
				b.WriteByte(c)
			}
			continue
		}
		if blockComment {
			if c == '*' && next == '/' {
				blockComment = false
				i++
			}
			continue
		}
		if quote != 0 {
			b.WriteByte(c)
			if c == '\\' && quote == '\'' {
				if i+1 < len(script) {
					i++
					b.WriteByte(script[i])
				}
				continue
			}
			if c == quote {
				if next == quote {
					i++
					b.WriteByte(script[i])
					continue
				}
				quote = 0
			}
			continue
		}

		switch {
		case c == '-' && next == '-':
			lineComment = true
			i++
		case c == '#':
			lineComment = true
		case c == '/' && next == '*':
			blockComment = true
			i++
		case c == '\'' || c == '"' || c == '`':
			quote = c
			b.WriteByte(c)
		case c == ';':
			if stmt := strings.TrimSpace(b.String()); stmt != "" {
				out = append(out, stmt)
			}
			b.Reset()
		default:
			b.WriteByte(c)
		}
	}

	if quote != 0 {
		return nil, fmt.Errorf("unterminated %q quote", string(quote))
	}
	if blockComment {
		return nil, fmt.Errorf("unterminated block comment")
	}
	if stmt := strings.TrimSpace(b.String()); stmt != "" {
		out = append(out, stmt)
	}
	return out, nil
}
