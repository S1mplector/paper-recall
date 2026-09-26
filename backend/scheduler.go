package main

import (
	"fmt"
	"math"
)

const minute int64 = 60_000
const day int64 = 24 * 60 * minute
const maxInterval = 3650.0
const maxTimestamp int64 = 253402300799999 // End of year 9999; well within JS exact integers.
func validateSchedule(p Schedule) error {
	if math.IsNaN(p.Interval) || math.IsInf(p.Interval, 0) || math.IsNaN(p.Ease) || math.IsInf(p.Ease, 0) || p.Interval < 0 || p.Interval > maxInterval || (p.Ease != 0 && p.Ease < 1.3) || p.Ease > 3 || p.Due < 0 || p.Due > maxTimestamp || p.LastReview < 0 || p.LastReview > maxTimestamp || p.Reviews < 0 || p.Reviews > 1000000000 || p.Lapses < 0 || p.Lapses > p.Reviews || p.Step < 0 || p.Step > 1 {
		return fmt.Errorf("invalid stored schedule")
	}
	switch p.Phase {
	case "", "new", "learning", "review", "relearning":
	default:
		return fmt.Errorf("unknown learning phase %q", p.Phase)
	}
	return nil
}

type Schedule struct {
	Phase      string  `json:"phase"`
	Step       int     `json:"step"`
	Due        int64   `json:"due"`
	Interval   float64 `json:"intervalDays"`
	Ease       float64 `json:"ease"`
	Reviews    int     `json:"reviews"`
	Lapses     int     `json:"lapses"`
	LastReview int64   `json:"lastReview"`
}

func schedule(p Schedule, rating string, now int64) (Schedule, error) {
	if e := validateSchedule(p); e != nil {
		return p, e
	}
	if now < 0 || now > maxTimestamp-int64(maxInterval)*day || p.Reviews >= 1000000000 {
		return p, fmt.Errorf("review time or counter out of range")
	}
	if rating != "again" && rating != "hard" && rating != "good" && rating != "easy" {
		return p, fmt.Errorf("unknown rating")
	}
	n := p
	if n.Ease == 0 {
		n.Ease = 2.5
	}
	if n.Phase == "" {
		n.Phase = "new"
	}
	n.Reviews++
	n.LastReview = now
	graduate := func(days float64) {
		n.Phase = "review"
		n.Step = 0
		n.Interval = math.Min(maxInterval, math.Max(1, math.Round(days)))
		n.Due = now + int64(n.Interval)*day
	}
	switch n.Phase {
	case "new", "learning":
		n.Phase = "learning"
		switch rating {
		case "again":
			n.Step = 0
			n.Due = now + minute
		case "hard":
			if n.Step == 0 {
				n.Due = now + 6*minute
			} else {
				n.Due = now + 10*minute
			}
		case "good":
			if n.Step == 0 {
				n.Step = 1
				n.Due = now + 10*minute
			} else {
				graduate(1)
			}
		case "easy":
			graduate(4)
		}
	case "relearning":
		switch rating {
		case "again":
			n.Due = now + minute
		case "hard":
			n.Due = now + 15*minute
		case "good":
			graduate(n.Interval)
		case "easy":
			n.Ease = math.Min(3, n.Ease+0.15)
			graduate(math.Max(n.Interval+1, n.Interval*1.3))
		}
	case "review":
		late := math.Max(0, float64(now-p.Due)/float64(day))
		switch rating {
		case "again":
			n.Lapses++
			n.Ease = math.Max(1.3, n.Ease-0.2)
			n.Interval = math.Max(1, math.Round(n.Interval*0.5))
			n.Phase = "relearning"
			n.Step = 0
			n.Due = now + 10*minute
		case "hard":
			n.Ease = math.Max(1.3, n.Ease-0.15)
			graduate(math.Max(n.Interval+1, n.Interval*1.2))
		case "good":
			graduate(math.Max(n.Interval+1, (n.Interval+late/2)*n.Ease))
		case "easy":
			n.Ease = math.Min(3, n.Ease+0.15)
			graduate(math.Max(n.Interval+2, (n.Interval+late)*n.Ease*1.3))
		}
	default:
		return p, fmt.Errorf("unknown learning phase %q", n.Phase)
	}
	return n, nil
}
func intervalLabel(ms int64) string {
	if ms < 60*minute {
		return fmt.Sprintf("%dm", max(int64(1), ms/minute))
	}
	if ms < day {
		return fmt.Sprintf("%dh", ms/(60*minute))
	}
	return fmt.Sprintf("%dd", ms/day)
}
