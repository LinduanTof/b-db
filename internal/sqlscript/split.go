package sqlscript

import (
	"fmt"
	"strings"
)

func Split(script string) ([]string, error) {
	return SplitFor(script, "")
}

func SplitFor(script, dialect string) ([]string, error) {
	hashComment := dialect == "" || dialect == "mysql"
	postgres := dialect == "postgres"
	mysql := dialect == "mysql"

	delimiter := ";"
	var out []string
	var b strings.Builder
	var quote byte
	dollar := ""
	lineComment := false
	blockComment := false
	atLineStart := true

	flush := func() {
		if stmt := strings.TrimSpace(b.String()); stmt != "" {
			out = append(out, stmt)
		}
		b.Reset()
	}

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
				atLineStart = true
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
		if dollar != "" {
			if strings.HasPrefix(script[i:], dollar) {
				b.WriteString(dollar)
				i += len(dollar) - 1
				dollar = ""
				atLineStart = false
				continue
			}
			b.WriteByte(c)
			if c == '\n' {
				atLineStart = true
			} else if c != ' ' && c != '\t' {
				atLineStart = false
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
				atLineStart = false
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
			if c == '\n' {
				atLineStart = true
			} else {
				atLineStart = false
			}
			continue
		}

		if mysql && delimiter != ";" && strings.HasPrefix(script[i:], delimiter) {
			flush()
			i += len(delimiter) - 1
			atLineStart = false
			continue
		}

		if atLineStart {
			if c == ' ' || c == '\t' {
				b.WriteByte(c)
				continue
			}
			if c == '\n' {
				b.WriteByte(c)
				continue
			}
			if mysql && (c == 'd' || c == 'D') {
				lineEnd := strings.IndexByte(script[i:], '\n')
				if lineEnd < 0 {
					lineEnd = len(script) - i
				}
				line := strings.TrimSpace(script[i : i+lineEnd])
				if len(line) > len("DELIMITER") && strings.EqualFold(line[:len("DELIMITER")], "DELIMITER") &&
					(line[len("DELIMITER")] == ' ' || line[len("DELIMITER")] == '\t') {
					fields := strings.Fields(line)
					if len(fields) >= 2 {
						delimiter = fields[1]
					}
					b.Reset()
					i += lineEnd
					atLineStart = true
					continue
				}
			}
			atLineStart = false
		}

		switch {
		case c == '-' && next == '-':
			lineComment = true
			i++
		case c == '#' && hashComment:
			lineComment = true
		case c == '/' && next == '*':
			blockComment = true
			i++
		case postgres && c == '$':
			if tag, ok := dollarTag(script[i:]); ok {
				dollar = tag
				b.WriteString(tag)
				i += len(tag) - 1
				continue
			}
			b.WriteByte(c)
		case c == '\'' || c == '"' || c == '`':
			quote = c
			b.WriteByte(c)
		case c == '\n':
			b.WriteByte(c)
			atLineStart = true
		case c == ';' && delimiter == ";":
			flush()
		default:
			b.WriteByte(c)
		}
	}

	if quote != 0 {
		return nil, fmt.Errorf("unterminated %q quote", string(quote))
	}
	if dollar != "" {
		return nil, fmt.Errorf("unterminated dollar-quoted string")
	}
	if blockComment {
		return nil, fmt.Errorf("unterminated block comment")
	}
	flush()
	return out, nil
}

func dollarTag(s string) (string, bool) {
	if len(s) < 2 || s[0] != '$' {
		return "", false
	}
	for i := 1; i < len(s); i++ {
		switch c := s[i]; {
		case c == '$':
			return s[:i+1], true
		case c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9':
			continue
		default:
			return "", false
		}
	}
	return "", false
}
