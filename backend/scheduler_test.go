package main

import (
	"math"
	"testing"
)

func TestLearningAndRelearning(t *testing.T) {
	now := int64(1800000000000)
	p := Schedule{}
	for r, want := range map[string]int64{"again": minute, "hard": 6 * minute, "good": 10 * minute, "easy": 4 * day} {
		n, e := schedule(p, r, now)
		if e != nil || n.Due != now+want {
			t.Fatalf("%s: %+v %v", r, n, e)
		}
	}
	p, _ = schedule(p, "good", now)
	p, _ = schedule(p, "good", p.Due)
	if p.Phase != "review" || p.Interval != 1 {
		t.Fatal(p)
	}
	p, _ = schedule(p, "good", p.Due)
	if p.Interval != 3 {
		t.Fatal(p)
	}
	p, _ = schedule(p, "again", p.Due)
	if p.Phase != "relearning" || p.Lapses != 1 || p.Interval != 2 {
		t.Fatal(p)
	}
	p, _ = schedule(p, "good", p.Due)
	if p.Phase != "review" || p.Interval != 2 {
		t.Fatal(p)
	}
}
func TestSchedulerBoundsAndRatingOrder(t *testing.T) {
	now := int64(1800000000000)
	for _, phase := range []string{"new", "learning", "relearning", "review"} {
		for _, interval := range []float64{0, 1, 3, 15, 100, 3650} {
			for _, ease := range []float64{1.3, 2.5, 3} {
				p := Schedule{Phase: phase, Interval: interval, Ease: ease, Due: now, Reviews: 10}
				last := int64(0)
				for _, r := range []string{"again", "hard", "good", "easy"} {
					n, e := schedule(p, r, now)
					if e != nil || n.Due <= now || n.Ease < 1.3 || n.Ease > 3 || n.Interval > 3650 || math.IsNaN(n.Interval) {
						t.Fatalf("invalid %+v %v", n, e)
					}
					if n.Due < last && phase == "review" {
						t.Fatalf("rating order %+v", p)
					}
					last = n.Due
				}
			}
		}
	}
	if _, e := schedule(Schedule{}, "unknown", now); e == nil {
		t.Fatal("accepted invalid rating")
	}
}
