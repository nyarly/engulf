package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/tools/cover"
)

type x cover.ProfileBlock
type xp struct {
	l, r x
	n    int
}

func testMerge(t *testing.T, p *xp) []cover.ProfileBlock {
	bls := mergeBlockPair(cover.ProfileBlock(p.l), cover.ProfileBlock(p.r))
	assert.Len(t, bls, p.n, "\n  %#v\n  %#v", p.l, p.r)
	return bls
}

func TestMergePairs(t *testing.T) {
	pairs := []xp{
		xp{x{0, 0, 0, 0, 0, 0}, x{1, 1, 1, 1, 0, 0}, 0},
		xp{x{40, 60, 45, 24, 2, 0}, x{45, 24, 46, 26, 1, 0}, 0},
		xp{x{45, 24, 46, 26, 1, 0}, x{40, 60, 45, 24, 2, 0}, 0},
		xp{x{1, 1, 1, 1, 0, 0}, x{1, 1, 1, 1, 0, 0}, 1},
		xp{x{0, 0, 2, 2, 0, 0}, x{1, 1, 2, 2, 0, 0}, 2}, //same end
		xp{x{1, 1, 2, 2, 0, 0}, x{0, 0, 2, 2, 0, 0}, 2}, //same end
		xp{x{0, 0, 2, 2, 0, 0}, x{0, 0, 1, 1, 0, 0}, 2}, //same begin
		xp{x{0, 0, 1, 1, 0, 0}, x{0, 0, 2, 2, 0, 0}, 2}, //same begin
		xp{x{0, 0, 2, 2, 0, 0}, x{1, 1, 3, 3, 0, 0}, 3}, //left overlap
		xp{x{1, 1, 3, 3, 0, 0}, x{0, 0, 2, 2, 0, 0}, 3}, //right overlap
		xp{x{0, 0, 3, 3, 0, 0}, x{1, 1, 2, 2, 0, 0}, 3}, //left outer
		xp{x{1, 1, 2, 2, 0, 0}, x{0, 0, 3, 3, 0, 0}, 3}, //right outer
	}

	for _, p := range pairs {
		testMerge(t, &p)
	}
}
func lb(blks ...cover.ProfileBlock) []cover.ProfileBlock {
	return blks
}

func blk(start, end int) cover.ProfileBlock {
	return cover.ProfileBlock{StartLine: start, EndLine: end}
}

func bk(sl, sc, el, ec, s, c int) cover.ProfileBlock {
	return cover.ProfileBlock{sl, sc, el, ec, s, c}
}

func TestMergeProfiles(t *testing.T) {
	assert := assert.New(t)

	assert.Len(mergeBlocks(lb(blk(0, 0), blk(4, 4), blk(2, 2), blk(7, 8)), lb(blk(1, 1), blk(5, 5))), 6)
	assert.Len(mergeBlocks(lb(blk(0, 7)), lb(blk(1, 5))), 3)
	assert.Len(mergeBlocks(lb(blk(0, 7), blk(8, 9)), lb(blk(1, 5))), 4)
	assert.Len(mergeBlocks(lb(bk(30, 41, 32, 2, 1, 0)), lb(bk(35, 52, 38, 2, 2, 0))), 2)

	assert.Len(mergeBlocks(
		lb(
			bk(40, 60, 45, 24, 2, 0),
			bk(45, 24, 46, 26, 1, 0),
		),
		lb(
			bk(40, 60, 45, 24, 2, 0),
			bk(45, 24, 46, 26, 1, 0),
		),
	),
		2,
	)

	assert.Len(mergeBlocks(
		lb(
			bk(30, 41, 32, 2, 1, 0),
			bk(35, 52, 38, 2, 2, 0),
			bk(40, 60, 45, 24, 2, 0),
			bk(45, 24, 46, 26, 1, 0),
			bk(46, 26, 48, 9, 2, 0),
			bk(51, 2, 51, 24, 1, 0),
			bk(51, 24, 52, 26, 1, 0),
			bk(52, 26, 54, 9, 2, 0),
			bk(57, 2, 57, 24, 1, 0),
			bk(57, 24, 58, 23, 1, 0),
			bk(58, 23, 60, 9, 2, 0),
			bk(63, 2, 63, 24, 1, 0),
			bk(63, 24, 64, 33, 1, 0),
			bk(64, 33, 65, 39, 1, 0),
			bk(65, 39, 67, 5, 1, 0),
			bk(69, 3, 69, 27, 1, 0),
			bk(69, 27, 70, 33, 1, 0),
			bk(70, 33, 72, 5, 1, 0),
			bk(75, 2, 75, 11, 1, 0),
		),
		lb(
			bk(30, 41, 32, 2, 1, 0),
			bk(35, 52, 38, 2, 2, 0),
			bk(40, 60, 45, 24, 2, 0),
			bk(45, 24, 46, 26, 1, 0),
			bk(46, 26, 48, 9, 2, 0),
			bk(51, 2, 51, 24, 1, 0),
			bk(51, 24, 52, 26, 1, 0),
			bk(52, 26, 54, 9, 2, 0),
			bk(57, 2, 57, 24, 1, 0),
			bk(57, 24, 58, 23, 1, 0),
			bk(58, 23, 60, 9, 2, 0),
			bk(63, 2, 63, 24, 1, 0),
			bk(63, 24, 64, 33, 1, 0),
			bk(64, 33, 65, 39, 1, 0),
			bk(65, 39, 67, 5, 1, 0),
			bk(69, 3, 69, 27, 1, 0),
			bk(69, 27, 70, 33, 1, 0),
			bk(70, 33, 72, 5, 1, 0),
			bk(75, 2, 75, 11, 1, 0),
		),
	),
		19,
	)
}
