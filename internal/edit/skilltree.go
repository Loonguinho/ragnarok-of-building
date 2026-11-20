package edit

import "maps"

// Skill is what the tree rules need to know about one skill.
type Skill struct {
	Max   int
	Needs [][2]int // prerequisite skill ID, level
	Group int      // index into the tree's point groups
	Free  bool     // quest/soul skills cost no points
}

// Tree is a job's skill tree as the client plays it: click +1, shift+click
// max, right-click -1, shift+right-click 0. Raising a skill fills its
// prerequisites; lowering one below what another needs resets dependents.
type Tree struct {
	Skills map[int]Skill
	// Budgets are the skill points of each class step, in class order; -1
	// means unknown (no job tables), and then anything fits.
	Budgets []int
}

// Used is the points spent per class step.
func (t Tree) Used(levels map[int]int) []int {
	used := make([]int, len(t.Budgets))
	for id, n := range levels {
		s, ok := t.Skills[id]
		if ok && !s.Free && s.Group >= 0 && s.Group < len(used) {
			used[s.Group] += n
		}
	}
	return used
}

// Fits reports whether the levels fit the budgets. A class step's points
// can go to its own or an earlier step's skills, never a later one's: the
// skills from each step on must fit the points from that step on.
func (t Tree) Fits(levels map[int]int) bool {
	for _, b := range t.Budgets {
		if b < 0 {
			return true
		}
	}
	used := t.Used(levels)
	u, b := 0, 0
	for i := len(t.Budgets) - 1; i >= 0; i-- {
		u += used[i]
		b += t.Budgets[i]
		if u > b {
			return false
		}
	}
	return true
}

// Raise sets skill id to level to (capped at its max), raising its
// prerequisites first. It returns the new levels and false, with the levels
// unchanged, when the points don't allow it.
func (t Tree) Raise(levels map[int]int, id, to int) (map[int]int, bool) {
	lv := maps.Clone(levels)
	if lv == nil {
		lv = map[int]int{}
	}
	t.raise(lv, id, to, map[int]bool{})
	if !t.Fits(lv) {
		return levels, false
	}
	return lv, true
}

func (t Tree) raise(lv map[int]int, id, to int, seen map[int]bool) {
	s, ok := t.Skills[id]
	if !ok || seen[id] {
		return
	}
	seen[id] = true
	to = min(to, s.Max)
	if to <= lv[id] {
		return
	}
	for _, n := range s.Needs {
		if lv[n[0]] < n[1] {
			t.raise(lv, n[0], n[1], seen)
		}
	}
	lv[id] = to
}

// Lower sets skill id to level to (0 removes it); skills whose
// prerequisites are no longer met are reset.
func (t Tree) Lower(levels map[int]int, id, to int) map[int]int {
	lv := maps.Clone(levels)
	if lv == nil || to >= lv[id] {
		return lv
	}
	if to <= 0 {
		delete(lv, id)
	} else {
		lv[id] = to
	}
	for changed := true; changed; {
		changed = false
		for sid := range lv {
			if t.Locked(lv, sid) {
				delete(lv, sid)
				changed = true
			}
		}
	}
	return lv
}

// Locked reports whether a skill's prerequisites are not met (it shows
// dimmed in the tree).
func (t Tree) Locked(levels map[int]int, id int) bool {
	for _, n := range t.Skills[id].Needs {
		if _, known := t.Skills[n[0]]; known && levels[n[0]] < n[1] {
			return true
		}
	}
	return false
}

// Max raises a skill as high as the points allow (shift+click).
func (t Tree) Max(levels map[int]int, id int) (map[int]int, bool) {
	s, ok := t.Skills[id]
	if !ok {
		return levels, false
	}
	for to := s.Max; to > levels[id]; to-- {
		if lv, ok := t.Raise(levels, id, to); ok {
			return lv, true
		}
	}
	return levels, false
}
