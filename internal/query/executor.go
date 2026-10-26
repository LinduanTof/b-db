package query

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/LinduanTof/dbrix/internal/db"
)

type Executor struct {
	Conn        db.Conn
	MaxRows     int
	AlwaysQuery bool
}

type Outcome struct {
	Result  *db.ResultSet
	Exec    *db.ExecResult
	Elapsed time.Duration
	Err     error
}

func (e Executor) Run(ctx context.Context, stmt string) Outcome {
	start := time.Now()
	if e.AlwaysQuery || IsQuery(stmt) {
		rs, err := e.Conn.Querier().Query(ctx, stmt, e.MaxRows)
		return Outcome{Result: rs, Err: err, Elapsed: time.Since(start)}
	}
	res, err := e.Conn.Querier().Exec(ctx, stmt)
	return Outcome{Exec: res, Err: err, Elapsed: time.Since(start)}
}

var queryKeywords = map[string]bool{
	"SELECT":   true,
	"WITH":     true,
	"SHOW":     true,
	"DESCRIBE": true,
	"DESC":     true,
	"EXPLAIN":  true,
	"VALUES":   true,
	"PRAGMA":   true,
	"TABLE":    true,
}

var dmlKeywords = map[string]bool{
	"INSERT":  true,
	"UPDATE":  true,
	"DELETE":  true,
	"MERGE":   true,
	"REPLACE": true,
	"UPSERT":  true,
}

var writeKeywords = map[string]bool{
	"INSERT": true, "UPDATE": true, "DELETE": true, "MERGE": true,
	"REPLACE": true, "UPSERT": true, "TRUNCATE": true, "CREATE": true,
	"ALTER": true, "DROP": true, "RENAME": true, "COMMENT": true,
	"GRANT": true, "REVOKE": true, "ATTACH": true, "DETACH": true,
	"VACUUM": true, "REINDEX": true, "ANALYZE": true, "CLUSTER": true,
	"REFRESH": true, "CALL": true, "DO": true, "COPY": true, "LOAD": true,
	"IMPORT": true, "EXPORT": true, "SET": true, "INTO": true,
	"EXECUTE": true, "INSTALL": true, "UNINSTALL": true, "BINLOG": true,
	"LOAD_EXTENSION": true, "WRITEFILE": true, "PURGE": true,

	// Redis write commands share the editor with SQL commands.
	"SETNX": true, "SETEX": true, "PSETEX": true, "DEL": true, "UNLINK": true,
	"INCR": true, "DECR": true, "INCRBY": true, "DECRBY": true, "INCRBYFLOAT": true,
	"APPEND": true, "SETRANGE": true, "SETBIT": true, "BITOP": true, "BITFIELD": true,
	"GETSET": true, "GETDEL": true, "GETEX": true,
	"EXPIRE": true, "PEXPIRE": true, "EXPIREAT": true, "PEXPIREAT": true, "PERSIST": true,
	"RENAMENX": true, "MOVE": true, "RESTORE": true,
	"MSET": true, "MSETNX": true, "HMSET": true, "HSET": true, "HSETNX": true,
	"HDEL": true, "HINCRBY": true, "HINCRBYFLOAT": true,
	"HGETDEL": true, "HGETEX": true, "HSETEX": true,
	"HEXPIRE": true, "HPEXPIRE": true, "HEXPIREAT": true, "HPERSIST": true,
	"LPUSH": true, "RPUSH": true, "LPUSHX": true, "RPUSHX": true,
	"LPOP": true, "RPOP": true, "LSET": true, "LREM": true,
	"LTRIM": true, "LINSERT": true, "LMOVE": true, "BLMOVE": true,
	"RPOPLPUSH": true, "BRPOPLPUSH": true, "LMPOP": true, "BLMPOP": true,
	"SADD": true, "SREM": true, "SPOP": true, "SMOVE": true,
	"SINTERSTORE": true, "SUNIONSTORE": true, "SDIFFSTORE": true,
	"ZADD": true, "ZREM": true, "ZINCRBY": true,
	"ZREMRANGEBYRANK": true, "ZREMRANGEBYSCORE": true, "ZREMRANGEBYLEX": true,
	"ZUNIONSTORE": true, "ZINTERSTORE": true, "ZDIFFSTORE": true,
	"ZPOPMIN": true, "ZPOPMAX": true, "BZPOPMIN": true, "BZPOPMAX": true,
	"ZMPOP": true, "BZMPOP": true, "ZRANGESTORE": true,
	"PFADD": true, "PFMERGE": true,
	"XADD": true, "XDEL": true, "XTRIM": true, "XGROUP": true,
	"XACK": true, "XCLAIM": true, "XAUTOCLAIM": true, "XSETID": true,
	"EVAL": true, "EVALSHA": true, "FCALL": true, "SCRIPT": true, "FUNCTION": true,
	"GEOADD": true, "GEORADIUS": true, "GEORADIUSBYMEMBER": true,
	"FLUSHDB": true, "FLUSHALL": true, "SWAPDB": true, "MIGRATE": true,
	"CONFIG": true, "ACL": true, "DEBUG": true, "SHUTDOWN": true,
	"PUBLISH": true, "SPUBLISH": true,
}

// IsQuery reports whether stmt should be executed with Querier.Query instead
// of Querier.Exec. A WITH statement is a query unless its outer statement is
// a plain DML statement; RETURNING always makes a statement produce rows.
func IsQuery(stmt string) bool {
	tokens := tokenize(stmt)
	if len(tokens) == 0 {
		return false
	}
	first := tokens[0]
	if !queryKeywords[first] {
		return containsToken(tokens, "RETURNING")
	}
	if first != "WITH" {
		return true
	}
	outer := outerTokens(stmt)
	if len(outer) == 0 {
		return true
	}
	if containsToken(outer, "RETURNING") {
		return true
	}
	for _, token := range outer[1:] {
		if dmlKeywords[token] {
			return false
		}
		if token == "SELECT" || token == "VALUES" || token == "TABLE" {
			return true
		}
	}
	return true
}

// IsWrite is a conservative check used to enforce read-only connections. It
// matches write keywords outside strings, quoted identifiers and comments.
func IsWrite(stmt string) bool {
	for _, token := range tokenize(stmt) {
		if writeKeywords[token] {
			return true
		}
	}
	return false
}

// IsWriteFor applies database-specific write detection. Redis uses an
// allowlist of read commands, MongoDB rejects stages and operators that can
// write or execute scripts, and SQL additionally guards PRAGMA/EXECUTE.
func IsWriteFor(dialect, stmt string) bool {
	switch strings.ToLower(dialect) {
	case "redis":
		return IsRedisWrite(stmt)
	case "mongo":
		return IsMongoWrite(stmt)
	default:
		return IsSQLWrite(stmt)
	}
}

var redisReadCommands = map[string]bool{
	"GET": true, "MGET": true, "GETRANGE": true, "STRLEN": true,
	"EXISTS": true, "TYPE": true, "TTL": true, "PTTL": true,
	"SCAN": true, "HSCAN": true, "SSCAN": true, "ZSCAN": true,
	"KEYS": true, "RANDOMKEY": true, "DBSIZE": true,
	"INFO": true, "TIME": true, "LASTSAVE": true, "ROLE": true,
	"HGET": true, "HMGET": true, "HGETALL": true, "HKEYS": true,
	"HVALS": true, "HLEN": true, "HEXISTS": true, "HSTRLEN": true,
	"HRANDFIELD": true,
	"LRANGE":     true, "LLEN": true, "LINDEX": true, "LPOS": true,
	"SMEMBERS": true, "SCARD": true, "SISMEMBER": true, "SMISMEMBER": true,
	"SINTER": true, "SUNION": true, "SDIFF": true, "SINTERCARD": true,
	"SRANDMEMBER": true,
	"ZRANGE":      true, "ZREVRANGE": true, "ZRANGEBYSCORE": true,
	"ZREVRANGEBYSCORE": true, "ZRANGEBYLEX": true, "ZREVRANGEBYLEX": true,
	"ZCARD": true, "ZCOUNT": true, "ZSCORE": true, "ZMSCORE": true,
	"ZRANK": true, "ZREVRANK": true, "ZLEXCOUNT": true,
	"ZRANDMEMBER": true, "ZINTERCARD": true,
	"XRANGE": true, "XREVRANGE": true, "XLEN": true, "XREAD": true,
	"XINFO": true, "XPENDING": true,
	"PFCOUNT": true, "BITCOUNT": true, "BITPOS": true, "GETBIT": true,
	"BITFIELD_RO": true, "LCS": true,
	"GEOPOS": true, "GEODIST": true, "GEOHASH": true, "GEOSEARCH": true,
	"GEORADIUS_RO": true, "GEORADIUSBYMEMBER_RO": true,
	"OBJECT": true, "MEMORY": true, "EVAL_RO": true, "EVALSHA_RO": true,
	"FCALL_RO": true, "SORT_RO": true, "SORT": true, "SELECT": true,
	"PING": true, "ECHO": true, "COMMAND": true, "WAIT": true, "PUBSUB": true,
}

// IsRedisWrite treats known read-only Redis commands as safe and everything
// else as a write. SORT is read-only unless it carries STORE.
func IsRedisWrite(stmt string) bool {
	word := firstWord(stmt)
	if word == "" {
		return false
	}
	if !redisReadCommands[word] {
		return true
	}
	if word == "SORT" {
		return slices.Contains(tokenize(stmt), "STORE")
	}
	return false
}

// IsMongoWrite rejects aggregate stages that can write and operators that can
// execute server-side JavaScript. The JSON payload is decoded so escaped
// spellings such as $out are caught as well.
func IsMongoWrite(stmt string) bool {
	if i := strings.IndexAny(stmt, "[{"); i >= 0 {
		var payload any
		if err := json.Unmarshal([]byte(stmt[i:]), &payload); err == nil {
			return mongoPayloadWrite(payload)
		}
	}
	lower := strings.ToLower(stmt)
	markers := []string{
		"$out", "$merge", "$where", "$function", "$accumulator",
		`$out`, `$merge`, `$where`, `$function`, `$accumulator`,
	}
	return slices.ContainsFunc(markers, func(marker string) bool {
		return strings.Contains(lower, marker)
	})
}

func mongoPayloadWrite(v any) bool {
	switch x := v.(type) {
	case map[string]any:
		for key, child := range x {
			switch strings.ToLower(key) {
			case "$out", "$merge", "$where", "$function", "$accumulator":
				return true
			}
			if mongoPayloadWrite(child) {
				return true
			}
		}
	case []any:
		return slices.ContainsFunc(x, mongoPayloadWrite)
	}
	return false
}

var readOnlyPragmas = map[string]bool{
	"TABLE_INFO": true, "TABLE_XINFO": true, "TABLE_LIST": true,
	"INDEX_LIST": true, "INDEX_INFO": true, "INDEX_XINFO": true,
	"FOREIGN_KEY_LIST": true, "DATABASE_LIST": true, "COLLATION_LIST": true,
	"FUNCTION_LIST": true, "MODULE_LIST": true, "COMPILE_OPTIONS": true,
	"INTEGRITY_CHECK": true, "QUICK_CHECK": true, "FOREIGN_KEY_CHECK": true,
}

func IsSQLWrite(stmt string) bool {
	// MySQL executes the body of /*! ... */ comments, so inspect each body
	// instead of letting the tokenizer discard it as a comment.
	if slices.ContainsFunc(mysqlExecutableBodies(stmt), IsSQLWrite) {
		return true
	}
	tokens := tokenize(stmt)
	if containsToken(tokens, "EXECUTE") {
		return true
	}
	if len(tokens) > 0 && tokens[0] == "PRAGMA" {
		// More than one PRAGMA in the same editor query is treated as a
		// write: tokenize drops semicolons, so the target of a later PRAGMA
		// cannot be attributed reliably.
		if slices.Contains(tokens[1:], "PRAGMA") {
			return true
		}
		if len(tokens) < 2 {
			return true
		}
		readOnly := readOnlyPragmas[tokens[1]]
		if !readOnly && len(tokens) > 2 {
			readOnly = readOnlyPragmas[tokens[2]]
		}
		if !readOnly {
			return true
		}
		return IsWrite(stmt)
	}
	// A PRAGMA after another statement is also conservatively rejected.
	if containsToken(tokens, "PRAGMA") {
		return true
	}
	return IsWrite(stmt)
}

func firstWord(stmt string) string {
	fields := strings.Fields(stmt)
	if len(fields) == 0 {
		return ""
	}
	return strings.ToUpper(fields[0])
}

func mysqlExecutableBodies(stmt string) []string {
	var out []string
	for i := 0; i < len(stmt); {
		j := strings.Index(stmt[i:], "/*!")
		if j < 0 {
			break
		}
		start := i + j + 3
		end := strings.Index(stmt[start:], "*/")
		if end < 0 {
			out = append(out, stmt[start:])
			break
		}
		out = append(out, stmt[start:start+end])
		i = start + end + 2
	}
	return out
}

func containsToken(tokens []string, want string) bool {
	return slices.Contains(tokens, want)
}

// outerTokens is like tokenize but only keeps identifiers at parenthesis depth
// zero, which lets WITH statements distinguish CTE bodies from the outer query.
func outerTokens(stmt string) []string {
	var out []string
	depth := 0
	for i := 0; i < len(stmt); {
		c := stmt[i]
		switch {
		case c == '-' && i+1 < len(stmt) && stmt[i+1] == '-':
			i = skipLine(stmt, i+2)
		case c == '/' && i+1 < len(stmt) && stmt[i+1] == '*':
			i = skipBlock(stmt, i+2)
		case c == '\'' || c == '"' || c == '`':
			i = skipQuoted(stmt, i, c)
		case c == '$':
			if next, ok := skipDollarQuoted(stmt, i); ok {
				i = next
			} else {
				i++
			}
		case c == '(':
			depth++
			i++
		case c == ')':
			if depth > 0 {
				depth--
			}
			i++
		case c == ';' || c == ',':
			i++
		default:
			if isIdentStart(rune(c)) {
				start := i
				for i < len(stmt) && isIdentPart(rune(stmt[i])) {
					i++
				}
				if depth == 0 {
					out = append(out, strings.ToUpper(stmt[start:i]))
				}
				continue
			}
			i++
		}
	}
	return out
}

// tokenize returns uppercase keywords and identifiers found outside comments,
// string literals, quoted identifiers and dollar-quoted bodies. Parentheses are
// skipped so a parenthesized SELECT is still recognized as a query.
func tokenize(stmt string) []string {
	var out []string
	for i := 0; i < len(stmt); {
		c := stmt[i]
		switch {
		case c == '-' && i+1 < len(stmt) && stmt[i+1] == '-':
			i = skipLine(stmt, i+2)
		case c == '/' && i+1 < len(stmt) && stmt[i+1] == '*':
			i = skipBlock(stmt, i+2)
		case c == '\'' || c == '"' || c == '`':
			i = skipQuoted(stmt, i, c)
		case c == '$':
			if next, ok := skipDollarQuoted(stmt, i); ok {
				i = next
			} else {
				i++
			}
		case c == '(' || c == ')' || c == ';' || c == ',':
			i++
		default:
			if isIdentStart(rune(c)) {
				start := i
				for i < len(stmt) && isIdentPart(rune(stmt[i])) {
					i++
				}
				out = append(out, strings.ToUpper(stmt[start:i]))
				continue
			}
			i++
		}
	}
	return out
}

func isIdentStart(r rune) bool {
	return unicode.IsLetter(r) || r == '_'
}

func isIdentPart(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '$'
}

func skipLine(s string, i int) int {
	if j := strings.IndexByte(s[i:], '\n'); j >= 0 {
		return i + j + 1
	}
	return len(s)
}

func skipBlock(s string, i int) int {
	if j := strings.Index(s[i:], "*/"); j >= 0 {
		return i + j + 2
	}
	return len(s)
}

func skipQuoted(s string, i int, quote byte) int {
	i++
	for i < len(s) {
		if s[i] == '\\' && quote == '\'' {
			i += 2
			continue
		}
		if s[i] == quote {
			if i+1 < len(s) && s[i+1] == quote {
				i += 2
				continue
			}
			return i + 1
		}
		i++
	}
	return len(s)
}

func skipDollarQuoted(s string, i int) (int, bool) {
	j := i + 1
	for j < len(s) && isTagPart(rune(s[j])) {
		j++
	}
	if j >= len(s) || s[j] != '$' {
		return i, false
	}
	delim := s[i : j+1]
	if end := strings.Index(s[j+1:], delim); end >= 0 {
		return j + 1 + end + len(delim), true
	}
	return len(s), true
}

func isTagPart(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}
