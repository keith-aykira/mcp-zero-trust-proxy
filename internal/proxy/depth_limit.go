package proxy

import (
	"encoding/json"
	"fmt"
)

const maxJSONDepth = 32

// LimitedUnmarshaler is a json.Unmarshal alternative that enforces a
// maximum JSON nesting depth to avoid stack overflow on deeply nested payloads.
type LimitedUnmarshaler struct {
	b        []byte
	maxDepth int
}

// NewLimitedUnmarshaler creates an unmarshaler bound to the supplied byte slice.
func NewLimitedUnmarshaler(b []byte, maxDepth int) *LimitedUnmarshaler {
	if maxDepth <= 0 {
		maxDepth = maxJSONDepth
	}
	return &LimitedUnmarshaler{b: b, maxDepth: maxDepth}
}

// Unmarshal decodes the JSON into v, erroring if nesting exceeds MaxDepth.
func (l *LimitedUnmarshaler) Unmarshal(v interface{}) error {
	data := skipWSBytes(l.b, 0)
	if len(data) == 0 {
		return fmt.Errorf("empty input")
	}
	if err := l.checkDepth(data, 0); err != nil {
		return err
	}
	// Depth ok; delegate to std json.Unmarshal
	return json.Unmarshal(l.b, v)
}

func (l *LimitedUnmarshaler) checkDepth(data []byte, depth int) error {
	if depth > l.maxDepth {
		return fmt.Errorf("JSON nesting depth %d exceeds maximum %d", depth, l.maxDepth)
	}

	i := 0
	if i >= len(data) {
		return nil
	}

	switch data[i] {
	case '[':
		_, ok := walkArray(data, i+1, depth+1, l.maxDepth)
		if !ok {
			return fmt.Errorf("invalid JSON array")
		}
	case '{':
		_, ok := walkObject(data, i+1, depth+1, l.maxDepth)
		if !ok {
			return fmt.Errorf("invalid JSON object")
		}
	default:
		// Primitive at top level is fine
	}
	return nil
}

func skipWSBytes(data []byte, i int) []byte {
	for i < len(data) && isWS(data[i]) {
		i++
	}
	return data[i:]
}

func isWS(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

func skipWS(data []byte, i int) int {
	for i < len(data) && isWS(data[i]) {
		i++
	}
	return i
}

func walkArray(data []byte, i int, depth int, maxDepth int) (int, bool) {
	if depth > maxDepth {
		return i, false
	}

	i = skipWS(data, i)
	if i < len(data) && data[i] == ']' {
		return i + 1, true
	}

	for {
		i = skipWS(data, i)
		var ok bool
		i, ok = walkValue(data, i, depth, maxDepth)
		if !ok {
			return i, false
		}

		i = skipWS(data, i)
		if i < len(data) && data[i] == ']' {
			return i + 1, true
		}
		if i >= len(data) || data[i] != ',' {
			return i, false
		}
		i++
	}
}

func walkObject(data []byte, i int, depth int, maxDepth int) (int, bool) {
	if depth > maxDepth {
		return i, false
	}

	i = skipWS(data, i)
	if i < len(data) && data[i] == '}' {
		return i + 1, true
	}

	for {
		i = skipWS(data, i)
		if i >= len(data) || data[i] != '"' {
			return i, false
		}
		var ok bool
		i, ok = skipString(data, i+1)
		if !ok {
			return i, false
		}

		i = skipWS(data, i)
		if i >= len(data) || data[i] != ':' {
			return i, false
		}
		i++

		i, ok = walkValue(data, i, depth, maxDepth)
		if !ok {
			return i, false
		}

		i = skipWS(data, i)
		if i < len(data) && data[i] == '}' {
			return i + 1, true
		}
		if i >= len(data) || data[i] != ',' {
			return i, false
		}
		i++
	}
}

func walkValue(data []byte, i int, depth int, maxDepth int) (int, bool) {
	if depth > maxDepth {
		return i, false
	}

	i = skipWS(data, i)
	if i >= len(data) {
		return i, false
	}

	switch data[i] {
	case '[':
		return walkArray(data, i+1, depth+1, maxDepth)
	case '{':
		return walkObject(data, i+1, depth+1, maxDepth)
	case '"':
		return skipString(data, i+1)
	default:
		// Primitive: true, false, null, number
		for i < len(data) {
			c := data[i]
			if c == ',' || c == ']' || c == '}' || isWS(c) {
				break
			}
			i++
		}
		return i, true
	}
}

func skipString(data []byte, i int) (int, bool) {
	for i < len(data) {
		if data[i] == '\\' {
			i += 2
			continue
		}
		if data[i] == '"' {
			return i + 1, true
		}
		i++
	}
	return i, false
}
