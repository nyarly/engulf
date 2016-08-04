package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"golang.org/x/tools/cover"

	"github.com/docopt/docopt-go"
	"github.com/nyarly/coerce"
)

type (
	blank  struct{}
	result struct {
		j *exec.Cmd
		o []byte
		e error
	}
	options struct {
		verbose         bool
		short           bool
		parallel        bool
		timeout         time.Duration
		covermode       string
		coverpkg        string
		maxJobs         uint
		coverdir        string
		packageSelector string
		exclude         string
		mergeBase       string
		onlyMerge       bool
	}
)

const (
	version   = `0.1`
	docstring = `Multiple-package coverage runner for Go
Usage: engulf [options] <package-selector>

Options:
	-v, --verbose                           Passed through to go test
	-s, --short                             Passed through to go test
	-p, --parallel                          Passed through to go test
	-t=<timeout>, --timeout=<timeout>       Passed through to go test [default: 10m]
	--covermode=<mode>                      Passed directly to go test [default: count]
	--coverpkg=<pkg>                        Passed directly to go test - defaults to <package-selector>
	-j=<n>, --max-jobs=<n>                  Run at most <n> test processes at once [default: 3]
	--coverdir=<dir>                        Storage dir for cover profiles [default: /tmp]
	-x=<pattern>, --exclude=<pattern>       comma separated <patterns> to exclude from coverage list [default: vendor/]
	--merge-base=<filename>                 base name to use for merging coverage
	--only-merge                            Don't do coverage, just merge coverage results
`
)

var (
	nothing   = blank{}
	pkgWarnRE = regexp.MustCompile(`(?m)^warning: no packages being tested depend on \S+$\n`)
	pathSep   = regexp.MustCompile(`/`)
)

func main() {
	log.Print("hit")
	opts := options{
		coverdir: "/tmp",
	}

	parsed, err := docopt.Parse(docstring, nil, true, version, false, false)
	log.Print(parsed, err)
	if err != nil {
		log.Fatal("parse:", err)
	}

	err = coerce.Struct(&opts, parsed, "-%s", "--%s", "<%s>")
	if err != nil {
		log.Fatal("coerce: ", err)
	}
	log.Printf("%+v", opts)

	gl := exec.Command("go", "list", opts.packageSelector)
	list, err := gl.CombinedOutput()
	if err != nil {
		fmt.Print(string(list))
		os.Exit(1)
	}
	pkgs := strings.Split(string(list), "\n")
	pkgs = pkgs[:len(pkgs)-1]

	excludeRE := regexp.MustCompile(strings.Join(strings.Split(opts.exclude, ","), "|"))
	for i := 0; i < len(pkgs); {
		if excludeRE.MatchString(pkgs[i]) {
			pkgs[i] = pkgs[len(pkgs)-1]
			pkgs = pkgs[:len(pkgs)-1]
		} else {
			i++
		}
	}

	if !opts.onlyMerge {
		if opts.coverpkg == "" {
			opts.coverpkg = strings.Join(pkgs, ",")
		}

		var covArgs []string

		if opts.verbose {
			covArgs = append(covArgs, "-v")
		}
		if opts.short {
			covArgs = append(covArgs, "-short")
		}
		if opts.parallel {
			covArgs = append(covArgs, "-parallel")
		}

		covJobs := make(chan *exec.Cmd, opts.maxJobs)
		startC := make(chan blank, opts.maxJobs)
		stopC := make(chan result, opts.maxJobs)

		go queueJobs(covJobs, opts.coverdir, opts.coverpkg, opts.covermode, covArgs, pkgs)

		for j := range covJobs {
			go runJob(j, startC, stopC)
		}

		var result error
		for i := 0; i < len(pkgs); i++ {
			res := <-stopC
			<-startC
			if res.e != nil {
				fmt.Print(formatTests(res))
			} else {
				fmt.Print(string(stripWarns(res.o)))
			}
		}

		if result != nil {
			fmt.Print(result)
			os.Exit(1)
		}
	}

	if opts.mergeBase != "" {
		mergedProfiles(opts.coverdir, opts.mergeBase, pkgs)
	}
}

func mergedProfiles(dir, basename string, pkgs []string) {
	merged := make(map[string]map[string]*cover.Profile)

	for _, pkg := range pkgs {
		profs, err := cover.ParseProfiles(profilepath(dir, pkg))
		if err != nil {
			log.Print(err)
			continue
		}

		for _, prof := range profs {
			moded, ok := merged[prof.Mode]
			if !ok {
				moded = make(map[string]*cover.Profile)
				merged[prof.Mode] = moded
			}
			filed, ok := moded[prof.FileName]
			if !ok {
				moded[prof.FileName] = &cover.Profile{
					FileName: prof.FileName,
					Mode:     prof.Mode,
					Blocks:   prof.Blocks,
				}
			} else {
				filed.Blocks = mergeBlocks(filed.Blocks, prof.Blocks)
			}
		}
	}

	for kind, m := range merged {
		var list []*cover.Profile
		for _, prf := range m {
			list = append(list, prf)
		}

		fname := filepath.Join(dir, kind+basename)
		writeCoverprofile(fname, kind, list)
	}
}

func writeCoverprofile(filename, mode string, list []*cover.Profile) error {
	pf, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer pf.Close()

	fmt.Fprintf(pf, "mode: %s\n", mode)

	for _, p := range list {
		for _, b := range p.Blocks {
			fmt.Fprintf(pf, "%s:%d.%d,%d.%d %d %d\n",
				p.FileName,
				b.StartLine, b.StartCol, b.EndLine, b.EndCol,
				b.NumStmt, b.Count,
			)
		}
	}
	return nil
}

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
		if left.EndLine < right.StartLine ||
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
	if left.StartLine > right.EndLine ||
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
			EndCol:    one.StartCol,
			EndLine:   one.StartLine,
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

func stripWarns(out []byte) []byte {
	return pkgWarnRE.ReplaceAll(out, []byte(""))
}

func formatTests(res result) string {
	return "BEGIN  " + res.j.Args[len(res.j.Args)-1] + "\n" + string(stripWarns(res.o))
}

func queueJobs(covJobs chan *exec.Cmd, dir, cpkg, mode string, covArgs, pkgs []string) {
	for _, p := range pkgs {
		path := profilepath(dir, p)
		os.MkdirAll(filepath.Dir(path), os.ModeDir|os.ModePerm)
		cov := exec.Command("go", "test", "--coverprofile="+path, "--coverpkg="+cpkg, "--covermode="+mode)
		cov.Args = append(cov.Args, covArgs...)
		cov.Args = append(cov.Args, p)
		covJobs <- cov
	}
	close(covJobs)
}

func runJob(j *exec.Cmd, start chan blank, stop chan result) {
	start <- nothing
	out, err := j.CombinedOutput()
	switch err.(type) {
	case *exec.ExitError:
		cmd := exec.Command("go", append([]string{"test"}, j.Args[5:]...)...) //brittle
		no, ne := cmd.CombinedOutput()
		if ne != nil {
			out, err = no, ne
		}
	}

	stop <- result{j: j, o: out, e: err}
}

func profilepath(dir, pkg string) string {
	return filepath.Join(dir, pathSep.ReplaceAllString(pkg, "-")) + ".coverprofile"
}
