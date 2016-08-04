package main

import "golang.org/x/tools/cover"

func mergeBlocks(left, right []cover.ProfileBlock) (merged []cover.ProfileBlock) {
	mls := make(map[cover.ProfileBlock]bool)
	mrs := make(map[cover.ProfileBlock]bool)

	for _, l := range left {
		for _, r := range right {
			overlap := mergeBlockPair(l, r)
			if len(overlap) > 0 {
				mls[l] = true
				mrs[r] = true
				merged = append(merged, overlap...)
			}
		}
	}
	for _, l := range left {
		if _, hit := mls[l]; !hit {
			merged = append(merged, l)
		}
	}

	for _, r := range right {
		if _, hit := mrs[r]; !hit {
			merged = append(merged, r)
		}
	}
	return merged
}

func mergeBlockPair(left, right cover.ProfileBlock) (merged []cover.ProfileBlock) {
	if left.StartLine < right.StartLine ||
		(left.StartLine == right.StartLine && left.StartCol < right.StartCol) {

		// No overlap: left is strictly before right
		if left.EndLine <= right.StartLine ||
			(left.EndLine == right.StartLine && left.EndCol <= right.StartCol) {
			return
		}

		// overlap, left is first
		if left.EndLine < right.EndLine ||
			(left.EndLine == right.EndLine && left.EndCol < right.EndCol) {
			return mergeOverlap(left, right)
		}

		// common end
		if left.EndLine == right.EndLine && left.EndCol == right.EndCol {
			return mergeCommonEnd(left, right)
		}

		// left completely covers right
		return mergeNested(right, left)
	}

	if left.StartLine == right.StartLine && left.StartCol == right.StartCol {
		if left.EndLine < right.EndLine ||
			(left.EndLine == right.EndLine && left.EndCol < right.EndCol) {
			return mergeCommonStart(left, right)
		}

		if left.EndLine == right.EndLine && left.EndCol == right.EndCol {
			return mergeSame(left, right)
		}

		return mergeCommonStart(right, left)
	}

	// No overlap: left is strictly after right
	if left.StartLine >= right.EndLine ||
		(left.StartLine == right.EndLine && left.StartCol >= right.EndCol) {
		return
	}

	// left starts within right

	if left.EndLine < right.EndLine ||
		(left.EndLine == right.EndLine && left.EndCol < right.EndCol) {
		return mergeNested(left, right)
	}

	if left.EndLine == right.EndLine && left.EndCol == right.EndCol {
		return mergeCommonEnd(right, left)
	}

	return mergeOverlap(right, left)
}

// same code covered
func mergeSame(one, other cover.ProfileBlock) []cover.ProfileBlock {
	return []cover.ProfileBlock{
		cover.ProfileBlock{
			StartCol:  one.StartCol,
			StartLine: one.StartLine,
			EndCol:    one.EndCol,
			EndLine:   one.EndLine,
			NumStmt:   one.NumStmt,
			Count:     one.Count + other.Count,
		},
	}
}

// inner is complete contained within outer
func mergeNested(inner, outer cover.ProfileBlock) []cover.ProfileBlock {
	diff := outer.NumStmt - inner.NumStmt
	// These can't be better than guesses
	x := diff / 2
	z := diff - x

	return []cover.ProfileBlock{
		cover.ProfileBlock{
			StartCol:  outer.StartCol,
			StartLine: outer.StartLine,
			EndCol:    inner.StartCol,
			EndLine:   inner.StartLine,
			NumStmt:   x,
			Count:     outer.Count,
		},
		cover.ProfileBlock{
			StartCol:  inner.StartCol,
			StartLine: inner.StartLine,
			EndCol:    inner.EndCol,
			EndLine:   inner.EndLine,
			NumStmt:   inner.NumStmt,
			Count:     inner.Count + outer.Count,
		},
		cover.ProfileBlock{
			StartCol:  inner.EndCol,
			StartLine: inner.EndLine,
			EndCol:    outer.EndCol,
			EndLine:   outer.EndLine,
			NumStmt:   z,
			Count:     outer.Count,
		},
	}
}

// tail of first overlaps with head of second
func mergeOverlap(first, second cover.ProfileBlock) []cover.ProfileBlock {
	sum := first.NumStmt + second.NumStmt
	// These can't be better than guesses
	x := sum / 3
	y := x
	z := sum - (2 * x)

	return []cover.ProfileBlock{
		cover.ProfileBlock{
			StartCol:  first.StartCol,
			StartLine: first.StartLine,
			EndCol:    second.StartCol,
			EndLine:   second.StartLine,
			NumStmt:   x,
			Count:     first.Count,
		},
		cover.ProfileBlock{
			StartCol:  first.StartCol,
			StartLine: first.StartLine,
			EndCol:    second.EndCol,
			EndLine:   second.EndLine,
			NumStmt:   y,
			Count:     first.Count + second.Count,
		},
		cover.ProfileBlock{
			StartCol:  first.EndCol,
			StartLine: first.EndLine,
			EndCol:    second.EndCol,
			EndLine:   second.EndLine,
			NumStmt:   z,
			Count:     second.Count,
		},
	}
}

// Same start, left is shorter : split where left ends
func mergeCommonStart(left, right cover.ProfileBlock) []cover.ProfileBlock {
	return []cover.ProfileBlock{
		cover.ProfileBlock{
			StartCol:  left.StartCol,
			StartLine: left.StartLine,
			EndCol:    left.EndCol,
			EndLine:   left.EndLine,
			NumStmt:   left.NumStmt,
			Count:     left.Count + right.Count,
		},
		cover.ProfileBlock{
			StartCol:  left.EndCol,
			StartLine: left.EndLine,
			EndCol:    right.EndCol,
			EndLine:   right.EndLine,
			NumStmt:   right.NumStmt - left.NumStmt,
			Count:     right.Count,
		},
	}
}

// Same end, second is shorter : split where second begins
func mergeCommonEnd(first, second cover.ProfileBlock) []cover.ProfileBlock {
	return []cover.ProfileBlock{
		cover.ProfileBlock{
			StartCol:  first.StartCol,
			StartLine: first.StartLine,
			EndCol:    second.StartCol,
			EndLine:   second.StartCol,
			NumStmt:   first.NumStmt - second.NumStmt,
			Count:     first.Count,
		},
		cover.ProfileBlock{
			StartCol:  second.StartCol,
			StartLine: second.StartCol,
			EndCol:    second.EndCol,
			EndLine:   second.EndCol,
			NumStmt:   second.NumStmt,
			Count:     first.Count + second.Count,
		},
	}
}
