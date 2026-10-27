package dice

import "testing"

func TestDieExplodes(t *testing.T) {
	r := NewRoller(&Fixed{Faces: []int{6, 6, 3}})
	if got := r.Die(); got != 15 {
		t.Fatalf("Die()=%d want 15", got)
	}
}

func TestDieNoExplosion(t *testing.T) {
	r := NewRoller(&Fixed{Faces: []int{5}})
	if got := r.Die(); got != 5 {
		t.Fatalf("got %d", got)
	}
}

func TestExplosionCapped(t *testing.T) {
	r := NewRoller(&Fixed{Faces: []int{6}}) // always 6
	if got := r.Die(); got != 6*(MaxExplosions+1) {
		t.Fatalf("got %d", got)
	}
}

func TestPoolNegative(t *testing.T) {
	if n := len(NewRoller(nil).Pool(-3)); n != 0 {
		t.Fatalf("len=%d", n)
	}
}

func TestSuccesses(t *testing.T) {
	cases := []struct {
		res    []int
		target int
		want   int
	}{
		{[]int{1, 2, 3, 4, 5, 6}, 4, 3},
		{[]int{1, 1, 1}, 1, 0}, // TN floor of 2: ones never succeed
		{[]int{2, 12}, 100, 0},
		{nil, 2, 0},
	}
	for _, c := range cases {
		if got := Successes(c.res, c.target); got != c.want {
			t.Errorf("Successes(%v,%d)=%d want %d", c.res, c.target, got, c.want)
		}
	}
}

func TestRandomRange(t *testing.T) {
	for i := 0; i < 1000; i++ {
		if v := (Random{}).D6(); v < 1 || v > 6 {
			t.Fatalf("out of range: %d", v)
		}
	}
}

func TestFixedClamp(t *testing.T) {
	f := &Fixed{Faces: []int{0, 9}}
	if f.D6() != 1 || f.D6() != 6 {
		t.Fatal("clamp failed")
	}
}
