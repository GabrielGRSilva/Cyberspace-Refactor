package format

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"cyberspace-refactor/internal/model"
)

// ErrNotDeck is returned when the data has no [Persona] section.
var ErrNotDeck = errors.New("format: no [Persona] section (not a valid DEK)")

// ErrNotNPC is returned when the data has no [Attributes] section.
var ErrNotNPC = errors.New("format: no [Attributes] section (not a valid NPC)")

var stageRe = regexp.MustCompile(`^(.*?)\s*\(([A-Za-z]+)\)$`)

// ParseUtility parses "Name-Rating-Size", "Name (Stage)-Rating-Size" or the
// data-file form "Name-Size".
func ParseUtility(s string) (model.Utility, error) {
	s = strings.TrimSpace(s)
	parts := strings.Split(s, "-")
	var nums []int
	for len(parts) > 1 && len(nums) < 2 {
		last := strings.TrimSpace(parts[len(parts)-1])
		v, err := strconv.Atoi(last)
		if err != nil || v < 0 || v > 1_000_000 {
			break
		}
		nums = append([]int{v}, nums...)
		parts = parts[:len(parts)-1]
	}
	if len(nums) == 0 {
		return model.Utility{}, fmt.Errorf("invalid utility %q", s)
	}
	u := model.Utility{Name: strings.TrimSpace(strings.Join(parts, "-"))}
	if u.Name == "" {
		return model.Utility{}, fmt.Errorf("invalid utility %q", s)
	}
	if len(nums) == 2 {
		u.Rating, u.Size = nums[0], nums[1]
	} else {
		u.Size = nums[0]
	}
	if m := stageRe.FindStringSubmatch(u.Name); m != nil {
		u.Name = m[1]
		u.Stage = strings.ToUpper(m[2])
		if len(u.Stage) >= 2 {
			u.Target = u.Stage[1:2]
		}
	}
	return u, nil
}

// FormatUtility renders a utility for a ProgramN= line.
func FormatUtility(u model.Utility) string {
	name := u.Name
	if u.Stage != "" {
		name += " (" + u.Stage + ")"
	}
	if u.Rating == 0 {
		return fmt.Sprintf("%s-%d", name, u.Size)
	}
	return fmt.Sprintf("%s-%d-%d", name, u.Rating, u.Size)
}

// ParseDeck parses a cyberdeck (.dek).
func ParseDeck(data []byte) (*model.Deck, []string, error) {
	doc := ParseINI(data)
	per := doc.Section("Persona")
	if per == nil {
		return nil, nil, ErrNotDeck
	}
	var warns []string
	d := &model.Deck{}
	p := &d.Persona
	p.Raw = map[string]string{}
	p.Model = per.GetDefault("Model", "Unknown")
	field := func(key string, dst *int) {
		if v, ok := per.Get(key); ok {
			p.Raw[key] = v
		}
		*dst = clamp(per.GetInt(key, 1), 0, 99)
	}
	field("MPCP", &p.MPCP)
	field("Bod", &p.Bod)
	field("Evasion", &p.Evasion)
	field("Masking", &p.Masking)
	field("Sensor", &p.Sensor)
	p.Damage = clamp(per.GetInt("Damage", 0), 0, 4)

	t := doc.Section("Technical Parameters")
	d.Hardening = clamp(t.getInt("Hardening", 0), 0, 99)
	d.ActiveMem = clamp(t.getInt("ActiveMem", 100), 0, 1_000_000)
	d.StorageMem = clamp(t.getInt("StorageMem", 500), 0, 10_000_000)
	d.LoadSpeed = clamp(t.getInt("Load", 20), 1, 1_000_000)
	d.IOSpeed = clamp(t.getInt("I/O", 20), 1, 1_000_000)
	d.Response = clamp(t.getInt("Response", 0), 0, 3)
	d.SatUplink = t.getInt("Satuplink", 0) != 0
	d.PureCyber = t.getInt("PureCyber", 0) != 0
	d.RealityFilter = t.getInt("RealityFilter", 0) != 0
	d.DisplayMode = clamp(t.getInt("DisplayMode", 1), 0, 2)

	for _, raw := range doc.Section("Utilities").withPrefix("Program") {
		if len(d.Utilities) >= model.MaxUtilities {
			warns = append(warns, "utility limit reached")
			break
		}
		u, err := ParseUtility(raw)
		if err != nil {
			warns = append(warns, err.Error())
			continue
		}
		d.Utilities = append(d.Utilities, u)
	}
	return d, warns, nil
}

// nil-safe wrappers so missing sections yield defaults.
func (s *Section) getInt(key string, def int) int {
	if s == nil {
		return def
	}
	return s.GetInt(key, def)
}

func (s *Section) withPrefix(p string) []string { return s.WithPrefix(p) }

// WriteDeck serialises a deck in the legacy format.
func WriteDeck(d *model.Deck) []byte {
	var sb strings.Builder
	w := func(format string, a ...any) { fmt.Fprintf(&sb, format+"\r\n", a...) }
	b2i := func(b bool) int {
		if b {
			return 1
		}
		return 0
	}
	p := d.Persona
	pf := func(key string, v int) {
		if raw, ok := p.Raw[key]; ok {
			if n, ok := LeadInt(raw); ok && n == v {
				w("%s=%s", key, oneLine(raw))
				return
			}
		}
		w("%s=%d", key, v)
	}
	w("[Persona]")
	w("Model=%s", oneLine(p.Model))
	pf("MPCP", p.MPCP)
	pf("Bod", p.Bod)
	pf("Evasion", p.Evasion)
	pf("Masking", p.Masking)
	pf("Sensor", p.Sensor)
	w("Damage=%d", p.Damage)
	w("")
	w("[Technical Parameters]")
	w("Hardening=%d", d.Hardening)
	w("ActiveMem=%d", d.ActiveMem)
	w("StorageMem=%d", d.StorageMem)
	w("Load=%d", d.LoadSpeed)
	w("I/O=%d", d.IOSpeed)
	w("Response=%d", d.Response)
	w("Satuplink=%d", b2i(d.SatUplink))
	w("PureCyber=%d", b2i(d.PureCyber))
	w("RealityFilter=%d", b2i(d.RealityFilter))
	w("DisplayMode=%d", d.DisplayMode)
	w("")
	w("[Utilities]")
	for i, u := range d.Utilities {
		w("Program%d=%s", i+1, FormatUtility(u))
	}
	return EncodeLegacy(sb.String())
}

// ParseNPC parses a decker (.npc).
func ParseNPC(data []byte) (*model.Decker, []string, error) {
	doc := ParseINI(data)
	attr := doc.Section("Attributes")
	if attr == nil {
		return nil, nil, ErrNotNPC
	}
	d := &model.Decker{}
	d.Name = doc.Section("Character").GetDefaultSafe("Name", "Unknown")
	g := func(key string, def int) int { return clamp(attr.GetInt(key, def), 0, 99) }
	d.Body = g("Body", 3)
	d.Quickness = g("Quickness", 3)
	d.Strength = g("Strength", 3)
	d.Charisma = g("Charisma", 3)
	d.Intelligence = g("Intelligence", 3)
	d.Willpower = g("Willpower", 3)
	d.Reaction = g("Reaction", (d.Quickness+d.Intelligence)/2)
	if v, ok := attr.Get("Reaction"); ok {
		if i := strings.IndexAny(v, "+"); i >= 0 {
			rest := strings.ToUpper(strings.TrimSpace(v[i+1:]))
			if n, ok := LeadInt(rest); ok && strings.Contains(rest, "D") {
				d.ReactionDice = clamp(n, 0, 9)
			}
		}
	}
	d.StunDamage = clamp(attr.GetInt("Stun Damage", 0), 0, 10)
	d.PhysDamage = clamp(attr.GetInt("Phys Damage", 0), 0, 10)

	sk := doc.Section("Skills")
	for _, k := range []string{"Computer", "Computer/Software/Decking", "Computer/Software"} {
		if v := sk.getInt(k, 0); v > d.Computer {
			d.Computer = clamp(v, 0, 99)
		}
	}
	d.Etiquette = clamp(sk.getInt("Etiquette/Matrix", 0), 0, 99)
	dp := doc.Section("Dice Pools")
	d.Karma = clamp(dp.getInt("Karma", 0), 0, 99)
	d.HackingPool = clamp(dp.getInt("Hacking", 0), 0, 99)
	d.TeamPool = clamp(dp.getInt("Team", 0), 0, 99)
	return d, nil, nil
}

// GetDefaultSafe is GetDefault tolerating a nil section.
func (s *Section) GetDefaultSafe(key, def string) string {
	if s == nil {
		return def
	}
	return s.GetDefault(key, def)
}

// WriteNPC serialises a decker in the legacy format.
func WriteNPC(d *model.Decker) []byte {
	var sb strings.Builder
	w := func(format string, a ...any) { fmt.Fprintf(&sb, format+"\r\n", a...) }
	w("[Character]")
	w("Name=%s", oneLine(d.Name))
	w("")
	w("[Attributes]")
	w("Body=%d", d.Body)
	w("Quickness=%d", d.Quickness)
	w("Strength=%d", d.Strength)
	w("Charisma=%d", d.Charisma)
	w("Intelligence=%d", d.Intelligence)
	w("Willpower=%d", d.Willpower)
	w("Reaction=%d+%dD", d.Reaction, d.ReactionDice)
	w("Stun Damage=%d", d.StunDamage)
	w("Phys Damage=%d", d.PhysDamage)
	w("")
	w("[Skills]")
	w("Computer=%d", d.Computer)
	w("Etiquette/Matrix=%d", d.Etiquette)
	w("")
	w("[Dice Pools]")
	w("Karma=%d", d.Karma)
	w("Hacking=%d", d.HackingPool)
	w("Team=%d", d.TeamPool)
	return EncodeLegacy(sb.String())
}
