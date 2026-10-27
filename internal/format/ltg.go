package format

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"cyberspace-refactor/internal/model"
)

// ErrNotLTG is returned when the data has no [Matrix] section.
var ErrNotLTG = errors.New("format: no [Matrix] section (not a valid LTG)")

var iceRe = regexp.MustCompile(`^(\S+)\s+(.+?)(?:\s*\(([^)]*)\))?-(\d+)(?:\s+(.*))?$`)
var fileRe = regexp.MustCompile(`(.+?)/(\d+)/(\d+)(?:/(\S+))?(?:\s+|$)`)

var triggerWords = map[string]bool{
	"ever": true, "passive": true, "active": true, "trigger": true, "triggered": true,
	"ptrigger": true, "atrigger": true, "ctrigger": true, "strigger": true,
}
var mobilityWords = map[string]bool{"immobile": true, "mobile": true, "pmobile": true, "amobile": true}

// ParseICE parses one ICEType value, e.g. "Gray Killer (M)-7 mobile active".
func ParseICE(s string) (model.ICE, error) {
	s = strings.TrimSpace(s)
	m := iceRe.FindStringSubmatch(s)
	if m == nil {
		return model.ICE{}, fmt.Errorf("invalid ICE %q", s)
	}
	rating, _ := strconv.Atoi(m[4])
	if rating > 99 {
		rating = 99
	}
	ice := model.ICE{Class: m[1], Type: strings.TrimSpace(m[2]), Stage: strings.TrimSpace(m[3]), Rating: rating}
	for _, tok := range strings.Fields(m[5]) {
		lt := strings.ToLower(tok)
		switch {
		case mobilityWords[lt]:
			ice.Mobility = lt
		case triggerWords[lt]:
			ice.Trigger = lt
		default:
			ice.Extra = append(ice.Extra, tok)
		}
	}
	return ice, nil
}

// FormatICE renders an ICE back to its legacy textual form.
func FormatICE(ice model.ICE) string {
	var sb strings.Builder
	sb.WriteString(ice.Class + " " + ice.Type)
	if ice.Stage != "" {
		sb.WriteString(" (" + ice.Stage + ")")
	}
	sb.WriteString("-" + strconv.Itoa(ice.Rating))
	if ice.Mobility != "" {
		sb.WriteString(" " + ice.Mobility)
	}
	if ice.Trigger != "" {
		sb.WriteString(" " + ice.Trigger)
	}
	for _, e := range ice.Extra {
		sb.WriteString(" " + e)
	}
	return sb.String()
}

// ParseFiles parses a Files= value ("Name/Mp/Nuyen[/password] ...").
func ParseFiles(s string) []model.File {
	s = strings.Join(strings.Fields(s), " ")
	var out []model.File
	pos := 0
	for pos < len(s) && len(out) < model.MaxFilesPerDS {
		m := fileRe.FindStringSubmatchIndex(s[pos:])
		if m == nil || m[0] != 0 {
			break
		}
		g := func(i int) string {
			if m[2*i] < 0 {
				return ""
			}
			return s[pos+m[2*i] : pos+m[2*i+1]]
		}
		size, _ := LeadInt(g(2))
		val, _ := LeadInt(g(3))
		out = append(out, model.File{Name: strings.TrimSpace(g(1)), Size: size, Value: val, Password: g(4)})
		pos += m[1]
	}
	return out
}

// FormatFiles renders files back to a Files= value.
func FormatFiles(files []model.File) string {
	parts := make([]string, 0, len(files))
	for _, f := range files {
		p := fmt.Sprintf("%s/%d/%d", strings.ReplaceAll(f.Name, "/", "-"), f.Size, f.Value)
		if f.Password != "" {
			p += "/" + f.Password
		}
		parts = append(parts, p)
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " ") + " "
}

func parseNodeType(n *model.Node, typ string) error {
	f := strings.Fields(typ)
	if len(f) == 0 {
		return errors.New("empty Type")
	}
	n.Kind = strings.ToUpper(f[0])
	if !model.ValidKind(n.Kind) {
		return fmt.Errorf("unknown node kind %q", f[0])
	}
	atoi := func(i int) int {
		if i < len(f) {
			v, _ := LeadInt(f[i])
			if v < 0 {
				v = 0
			}
			if v > 10000 {
				v = 10000
			}
			return v
		}
		return 0
	}
	if n.Kind == model.LNK {
		if len(f) < 2 {
			return errors.New("LNK without target file")
		}
		n.LinkFile = f[1]
		n.LinkEntry = atoi(2)
		n.X, n.Y = atoi(3), atoi(4)
		n.Color, n.Rating = model.Green, 4
		return nil
	}
	n.Color, n.Rating = model.Green, 4
	if len(f) > 1 {
		cr := strings.SplitN(f[1], "-", 2)
		if c, ok := model.ColorFromLetter(cr[0]); ok {
			n.Color = c
		}
		if len(cr) == 2 {
			if v, ok := LeadInt(cr[1]); ok {
				n.Rating = v
			}
		}
	}
	n.X, n.Y = atoi(2), atoi(3)
	return nil
}

// ParseLTG parses a Local Telecommunications Grid. Recoverable problems are
// returned as warnings; only a missing [Matrix] section is fatal.
func ParseLTG(data []byte) (*model.Matrix, []string, error) {
	doc := ParseINI(data)
	hdr := doc.Section("Matrix")
	if hdr == nil {
		return nil, nil, ErrNotLTG
	}
	var warns []string
	m := model.NewMatrix()
	m.Description = hdr.GetDefault("Description", "Matrix System")
	m.Entry = hdr.GetInt("Entry", 1)
	m.Range = clamp(hdr.GetInt("Range", 2), 0, 2)
	m.Alert = strings.ToLower(hdr.GetDefault("Alert", "none"))
	if m.Alert != "passive" && m.Alert != "active" {
		m.Alert = "none"
	}
	m.ShutdownTics = hdr.GetInt("Shutdowntics", 0)
	m.ShutdownPct = hdr.GetInt("Emergency", 0)
	m.ImagePath = hdr.GetDefault("LTGImagePath", "")
	m.ReadOnly = strings.EqualFold(hdr.GetDefault("Readonly", "No"), "yes")

	iceTotal := 0
	for i := range doc.Sections {
		sec := &doc.Sections[i]
		low := strings.ToLower(sec.Name)
		if !strings.HasPrefix(low, "node") {
			continue
		}
		id, err := strconv.Atoi(low[4:])
		if err != nil || id < 0 || id > model.MaxNodes {
			warns = append(warns, fmt.Sprintf("ignored section [%s]", sec.Name))
			continue
		}
		if _, dup := m.Nodes[id]; dup {
			warns = append(warns, fmt.Sprintf("duplicate [%s] ignored", sec.Name))
			continue
		}
		n := &model.Node{ID: id}
		if err := parseNodeType(n, sec.GetDefault("Type", "")); err != nil {
			warns = append(warns, fmt.Sprintf("node %d skipped: %v", id, err))
			continue
		}
		n.Message = sec.GetDefault("Mesg", "")
		seen := map[int]bool{}
		for _, part := range strings.Split(sec.GetDefault("Links", ""), "/") {
			v, ok := LeadInt(part)
			if !ok || v < 0 || v > model.MaxNodes || v == id || seen[v] {
				continue
			}
			if len(n.Links) >= model.MaxLinksPerNode {
				warns = append(warns, fmt.Sprintf("node %d: too many links, extra ignored", id))
				break
			}
			seen[v] = true
			n.Links = append(n.Links, v)
		}
		n.Files = ParseFiles(sec.GetDefault("Files", ""))
		for _, raw := range sec.WithPrefix("ICEType") {
			if len(n.ICE) >= model.MaxICEPerNode || iceTotal >= model.MaxICEPerMatrix {
				warns = append(warns, fmt.Sprintf("node %d: ICE limit reached", id))
				break
			}
			ice, err := ParseICE(raw)
			if err != nil {
				warns = append(warns, fmt.Sprintf("node %d: %v", id, err))
				continue
			}
			n.ICE = append(n.ICE, ice)
			iceTotal++
		}
		m.Nodes[id] = n
	}
	if len(m.Nodes) == 0 {
		warns = append(warns, "matrix has no nodes")
	}
	warns = append(warns, FixLinks(m)...)
	if _, ok := m.Nodes[m.Entry]; !ok && len(m.Nodes) > 0 {
		warns = append(warns, fmt.Sprintf("entry node %d missing; using node 1", m.Entry))
		m.Entry = 1
	}
	return m, warns, nil
}

// FixLinks drops links to missing nodes and adds missing back-links, as the
// original "error correction routine" did.
func FixLinks(m *model.Matrix) []string {
	var warns []string
	for _, id := range m.IDs() {
		n := m.Nodes[id]
		kept := n.Links[:0]
		for _, l := range n.Links {
			if _, ok := m.Nodes[l]; ok {
				kept = append(kept, l)
			} else {
				warns = append(warns, fmt.Sprintf("node %d: link to missing node %d removed", id, l))
			}
		}
		n.Links = kept
	}
	for _, id := range m.IDs() {
		for _, l := range m.Nodes[id].Links {
			t := m.Nodes[l]
			if !t.Linked(id) && len(t.Links) < model.MaxLinksPerNode {
				t.Links = append(t.Links, id)
				warns = append(warns, fmt.Sprintf("added missing back-link %d -> %d", l, id))
			}
		}
	}
	return warns
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// NodeTypeString renders the Type= value for a node.
func NodeTypeString(n *model.Node) string {
	if n.Kind == model.LNK {
		return fmt.Sprintf("LNK %s %d %d %d", n.LinkFile, n.LinkEntry, n.X, n.Y)
	}
	return fmt.Sprintf("%s %s-%d %d %d", n.Kind, n.Color, n.Rating, n.X, n.Y)
}

// WriteLTG serialises a matrix in the legacy format (CRLF, Windows-1252).
func WriteLTG(m *model.Matrix) []byte {
	var sb strings.Builder
	w := func(format string, a ...any) { fmt.Fprintf(&sb, format+"\r\n", a...) }
	maxID := 0
	for id := range m.Nodes {
		if id > maxID {
			maxID = id
		}
	}
	alert := m.Alert
	if alert == "" {
		alert = "none"
	}
	w("[Matrix]")
	w("Description=%s", oneLine(m.Description))
	w("Nodes=%d", maxID)
	w("Entry=%d", m.Entry)
	w("Range=%d", m.Range)
	w("Alert=%s", alert)
	if m.ShutdownTics > 0 {
		w("Shutdowntics=%d", m.ShutdownTics)
	}
	if m.ShutdownPct > 0 {
		w("Emergency=%d", m.ShutdownPct)
	}
	if m.ReadOnly {
		w("Readonly=Yes")
	} else {
		w("Readonly=No")
	}
	if m.ImagePath != "" {
		w("LTGImagePath=%s", oneLine(m.ImagePath))
	}
	ids := m.IDs()
	sort.Ints(ids)
	for _, id := range ids {
		n := m.Nodes[id]
		w("")
		w("[Node%d]", id)
		w("Type=%s", NodeTypeString(n))
		if n.Message != "" {
			w("Mesg=%s", oneLine(n.Message))
		}
		if len(n.Files) > 0 {
			w("Files=%s", FormatFiles(n.Files))
		}
		for i, ice := range n.ICE {
			key := "ICEType"
			if len(n.ICE) > 1 {
				key = "ICEType" + strconv.Itoa(i+1)
			}
			w("%s=%s", key, FormatICE(ice))
		}
		parts := make([]string, len(n.Links))
		for i, l := range n.Links {
			parts[i] = strconv.Itoa(l)
		}
		w("Links=%s", strings.Join(parts, "/"))
	}
	return EncodeLegacy(sb.String())
}

func oneLine(s string) string {
	s = strings.ReplaceAll(s, "\r", " ")
	return strings.ReplaceAll(s, "\n", " ")
}
