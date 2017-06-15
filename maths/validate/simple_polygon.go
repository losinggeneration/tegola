package validate

import (
	"sort"

	"github.com/terranodo/tegola/maths"
)

func xyorder(pt1, pt2 maths.Pt) int {

	switch {

	// Test the x-coord first
	case pt1.X > pt2.X:
		return 1
	case pt1.X < pt2.X:
		return -1

	// Test the y-coord second
	case pt1.Y > pt2.Y:
		return 1
	case pt1.Y < pt2.Y:
		return -1

	}

	// when you exclude all other possibilities, what remains  is...
	return 0 // they are the same point
}

func yxorder(pt1, pt2 maths.Pt) int {

	// Test the y-coord first
	switch {
	case pt1.Y > pt2.Y:
		return 1
	case pt1.Y < pt2.Y:
		return -1

	// Test the x-coord second
	case pt1.X > pt2.X:
		return 1
	case pt1.X < pt2.X:
		return -1
	}

	// when you exclude all other possibilities, what remains  is...
	return 0 // they are the same point
}

type eventType uint8

const (
	LEFT eventType = iota
	RIGHT
)

type event struct {
	edge     int
	edgeType eventType //
	ev       *maths.Pt // event vertex
}

type XYOrderedEventPtr []*event

func (a XYOrderedEventPtr) Len() int           { return len(a) }
func (a XYOrderedEventPtr) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a XYOrderedEventPtr) Less(i, j int) bool { return xyorder(*(a[i].ev), *(a[j].ev)) == -1 }

type eventQueue struct {
	edata []event
	eq    []*event // sorted list of event pointers
	ix    int      // index of next event in the queue
}

func (eq *eventQueue) Next() *event {
	if eq.ix >= len(eq.edata) {
		return nil
	}
	idx := eq.ix
	eq.ix++
	return eq.eq[idx]
}

func (eq *eventQueue) Complement(e *event) *event {
	idx := e.edge * 2
	if e.edgeType == LEFT {
		return eq.eq[idx+1]
	}
	return eq.eq[idx]
}

// Code adapted from http://geomalgorithms.com/a09-_intersect-3.html#simple_Polygon()
func NewEventQueue(segments []maths.Line) *eventQueue {

	ne := len(segments) * 2
	eq := new(eventQueue)
	eq.edata = make([]event, ne)
	eq.eq = make([]*event, ne)
	for i := range eq.edata {
		eq.eq[i] = &eq.edata[i]
	}

	// Initialize event queue with edge segment endpoints
	for i := range segments {
		idx := 2 * i
		eq.eq[idx].edge = i
		eq.eq[idx+1].edge = i
		eq.eq[idx].ev = &(segments[i][0])
		eq.eq[idx+1].ev = &(segments[i][1])
		if xyorder(segments[i][0], segments[i][1]) < 0 {
			eq.eq[idx].edgeType = LEFT
			eq.eq[idx+1].edgeType = RIGHT
		} else {
			eq.eq[idx].edgeType = RIGHT
			eq.eq[idx+1].edgeType = LEFT
		}
	}
	sort.Sort(XYOrderedEventPtr(eq.eq))
	return eq
}

func DoesIntersect(s1, s2 maths.Line) bool {

	as2 := s2.LeftRightMostAsLine()
	as1 := s1.LeftRightMostAsLine()

	lsign := as1.IsLeft(as2[0]) // s2 left point sign
	rsign := as1.IsLeft(as2[1]) // s2 right point sign
	if lsign*rsign > 0 {        // s2 endpoints have same sign  relative to s1
		return false // => on same side => no intersect is possible
	}

	lsign = as2.IsLeft(as1[0]) // s1 left point sign
	rsign = as2.IsLeft(as1[1]) // s1 right point sign
	if lsign*rsign > 0 {       // s1 endpoints have same sign  relative to s2
		return false // => on same side => no intersect is possible
	}
	// the segments s1 and s2 straddle each other
	return true //=> an intersect exists

}

func FindIntersects(segments []maths.Line, fn func(srcIdx, destIdx int, ptfn func() maths.Pt) bool) {

	eq := NewEventQueue(segments)
	ns := len(segments)
	if ns < 3 {
		return
	}
	var val struct{}

	isegmap := make(map[int]struct{})
	for ev := eq.Next(); ev != nil; ev = eq.Next() {

		_, ok := isegmap[ev.edge]

		if !ok {
			// have not seen this edge, let's add it to our list.
			isegmap[ev.edge] = val
			continue
		}

		// We have reached the end of a segment.
		// This is the left edge.
		delete(isegmap, ev.edge)
		if len(isegmap) == 0 {
			// no segments to test.
			continue
		}
		edge := segments[ev.edge]
		var segs = make([]int, 0, len(isegmap))
		for l := range isegmap {
			segs = append(segs, l)
		}

		for _, s := range segs {
			src, dest := (s+1)%ns, (ev.edge+1)%ns

			if ev.edge == s || src == ev.edge || dest == s {
				continue // no non-simple intersect since consecutive or the same line
			}
			sedge := segments[s]
			if !DoesIntersect(edge, sedge) {
				continue
			}

			ptfn := func() maths.Pt {
				pt, _ := maths.Intersect(edge, sedge)
				return pt
			}
			src, dest = ev.edge, s
			if src > dest {
				src, dest = dest, src
			}

			if !fn(src, dest, ptfn) {
				return
			}
		}
	}
	return
}

func IsSimple(segments []maths.Line) bool {
	var found bool = true
	FindIntersects(segments, func(_, _ int, _ func() maths.Pt) bool {
		found = false
		return false
	})
	return found
}
