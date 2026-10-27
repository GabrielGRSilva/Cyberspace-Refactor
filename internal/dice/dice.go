// Package dice implements Shadowrun 2nd Edition dice mechanics (exploding d6,
// success counting) behind an injectable random source so that every rule
// built on top of it is deterministic under test.
package dice

import (
	"math/rand/v2"
)

// MaxExplosions caps the "Rule of Six" re-rolls per die so a pathological
// random source can never loop forever.
const MaxExplosions = 50

// Source yields uniformly distributed integers in [1,6].
type Source interface {
	D6() int
}

// Random is the production Source, backed by math/rand/v2 (auto-seeded).
type Random struct{}

// D6 returns a random die face.
func (Random) D6() int { return rand.IntN(6) + 1 }

// Fixed is a deterministic Source that replays a sequence of faces and then
// repeats the last one. It exists for tests and for the manual-roll mode.
type Fixed struct {
	Faces []int
	pos   int
}

// D6 returns the next scripted face (clamped to 1..6).
func (f *Fixed) D6() int {
	if len(f.Faces) == 0 {
		return 1
	}
	i := f.pos
	if i >= len(f.Faces) {
		i = len(f.Faces) - 1
	} else {
		f.pos++
	}
	v := f.Faces[i]
	if v < 1 {
		v = 1
	}
	if v > 6 {
		v = 6
	}
	return v
}

// Roller rolls pools of dice using a Source.
type Roller struct {
	Src Source
}

// NewRoller returns a Roller using the given Source (Random if nil).
func NewRoller(src Source) *Roller {
	if src == nil {
		src = Random{}
	}
	return &Roller{Src: src}
}

// Die rolls a single die applying the Rule of Six (a 6 is re-rolled and added).
func (r *Roller) Die() int {
	total := 0
	for i := 0; i <= MaxExplosions; i++ {
		v := r.Src.D6()
		total += v
		if v != 6 {
			break
		}
	}
	return total
}

// Pool rolls n dice (n<0 is treated as 0) and returns each result.
func (r *Roller) Pool(n int) []int {
	if n < 0 {
		n = 0
	}
	out := make([]int, n)
	for i := range out {
		out[i] = r.Die()
	}
	return out
}

// Successes counts dice meeting or beating the target number. In SR2 a target
// number below 2 is treated as 2 (a natural 1 never succeeds on its own).
func Successes(results []int, target int) int {
	if target < 2 {
		target = 2
	}
	n := 0
	for _, v := range results {
		if v >= target {
			n++
		}
	}
	return n
}

// Test rolls n dice against a target and returns results and success count.
func (r *Roller) Test(n, target int) ([]int, int) {
	res := r.Pool(n)
	return res, Successes(res, target)
}
