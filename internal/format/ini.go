// Package format reads and writes the legacy Cyberspace file formats
// (.ltg, .dek, .npc) and their JSON equivalents. Parsers are tolerant (as the
// original was) but strictly bounded: they never panic and never allocate
// beyond the limits declared in package model.
package format

import (
	"bytes"
	"strings"
)

const (
	maxLines    = 200000
	maxSections = 4096
	maxLineLen  = 64 << 10
)

// Entry is one key=value pair, in file order.
type Entry struct{ Key, Value string }

// Section is an INI section. Entries keep duplicates and order.
type Section struct {
	Name    string
	Entries []Entry
}

// INI is a parsed Windows-style INI document.
type INI struct{ Sections []Section }

// cp1252 maps bytes 0x80-0x9F (Windows-1252) to runes; other bytes are Latin-1.
var cp1252 = [32]rune{
	0x20AC, 0x81, 0x201A, 0x0192, 0x201E, 0x2026, 0x2020, 0x2021, 0x02C6, 0x2030, 0x0160, 0x2039, 0x0152, 0x8D, 0x017D, 0x8F,
	0x90, 0x2018, 0x2019, 0x201C, 0x201D, 0x2022, 0x2013, 0x2014, 0x02DC, 0x2122, 0x0161, 0x203A, 0x0153, 0x9D, 0x017E, 0x0178,
}

// DecodeLegacy converts legacy Windows-1252 bytes to a UTF-8 string, dropping
// NUL bytes and carriage returns' companions safely. Valid UTF-8 BOM is removed.
func DecodeLegacy(b []byte) string {
	b = bytes.TrimPrefix(b, []byte{0xEF, 0xBB, 0xBF})
	var sb strings.Builder
	sb.Grow(len(b))
	for _, c := range b {
		switch {
		case c == 0:
			continue
		case c >= 0x80 && c < 0xA0:
			sb.WriteRune(cp1252[c-0x80])
		default:
			sb.WriteRune(rune(c))
		}
	}
	return sb.String()
}

// EncodeLegacy converts UTF-8 text to Windows-1252 bytes; unmappable runes
// become '?'.
func EncodeLegacy(s string) []byte {
	out := make([]byte, 0, len(s))
	for _, r := range s {
		switch {
		case r < 0x80, r >= 0xA0 && r <= 0xFF:
			out = append(out, byte(r))
		default:
			found := false
			for i, m := range cp1252 {
				if m == r {
					out = append(out, byte(0x80+i))
					found = true
					break
				}
			}
			if !found {
				out = append(out, '?')
			}
		}
	}
	return out
}

// ParseINI parses legacy INI text. Lines without '=' continue the previous
// value (the original allowed Files= and Mesg= to wrap). Comments start with
// '#' or ';'.
func ParseINI(data []byte) *INI {
	text := DecodeLegacy(data)
	doc := &INI{}
	cur := -1
	count := 0
	for _, line := range strings.Split(text, "\n") {
		count++
		if count > maxLines {
			break
		}
		if len(line) > maxLineLen {
			line = line[:maxLineLen]
		}
		line = strings.TrimSpace(line)
		if line == "" || line[0] == '#' || line[0] == ';' {
			continue
		}
		if line[0] == '[' {
			if end := strings.IndexByte(line, ']'); end > 0 {
				if len(doc.Sections) >= maxSections {
					break
				}
				doc.Sections = append(doc.Sections, Section{Name: strings.TrimSpace(line[1:end])})
				cur = len(doc.Sections) - 1
				continue
			}
		}
		if cur < 0 {
			continue
		}
		if i := strings.IndexByte(line, '='); i > 0 {
			doc.Sections[cur].Entries = append(doc.Sections[cur].Entries, Entry{
				Key: strings.TrimSpace(line[:i]), Value: strings.TrimSpace(line[i+1:]),
			})
			continue
		}
		// Continuation of the previous value.
		es := doc.Sections[cur].Entries
		if len(es) > 0 {
			last := &doc.Sections[cur].Entries[len(es)-1]
			if last.Value == "" {
				last.Value = line
			} else {
				last.Value += "\n" + line
			}
		}
	}
	return doc
}

// Section returns the first section named name (case-insensitive).
func (d *INI) Section(name string) *Section {
	for i := range d.Sections {
		if strings.EqualFold(d.Sections[i].Name, name) {
			return &d.Sections[i]
		}
	}
	return nil
}

// Get returns the first value for key (case-insensitive).
func (s *Section) Get(key string) (string, bool) {
	if s == nil {
		return "", false
	}
	for _, e := range s.Entries {
		if strings.EqualFold(e.Key, key) {
			return e.Value, true
		}
	}
	return "", false
}

// GetDefault returns the value for key or def.
func (s *Section) GetDefault(key, def string) string {
	if v, ok := s.Get(key); ok {
		return v
	}
	return def
}

// GetInt returns the leading integer of the value for key, or def.
func (s *Section) GetInt(key string, def int) int {
	v, ok := s.Get(key)
	if !ok {
		return def
	}
	n, ok := LeadInt(v)
	if !ok {
		return def
	}
	return n
}

// WithPrefix returns values whose key starts with prefix (case-insensitive),
// in file order.
func (s *Section) WithPrefix(prefix string) []string {
	if s == nil {
		return nil
	}
	var out []string
	p := strings.ToLower(prefix)
	for _, e := range s.Entries {
		if strings.HasPrefix(strings.ToLower(e.Key), p) {
			out = append(out, e.Value)
		}
	}
	return out
}

// LeadInt parses the leading (optionally signed) integer of s, so values such
// as "6+0-0-0" or "5+1D" yield 6 and 5. Values are clamped to a sane range.
func LeadInt(s string) (int, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	i, neg := 0, false
	if s[0] == '-' || s[0] == '+' {
		neg = s[0] == '-'
		i++
	}
	start := i
	n := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		if n < 1_000_000_000 {
			n = n*10 + int(s[i]-'0')
		}
		i++
	}
	if i == start {
		return 0, false
	}
	if neg {
		n = -n
	}
	return n, true
}
