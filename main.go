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
		exclude         []string
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
-j=<n>, --max-jobs=<n>                  Run at most <n> test processes at once
--coverdir=<dir>                        Storage dir for cover profiles [default: /tmp]
-x=<pattern>, --exclude=<pattern>...    <patterns> to exclude from coverage list
`
)

var (
	nothing   = blank{}
	pkgWarnRE = regexp.MustCompile(`(?m)^warning: no packages being tested depend on \S+$\n`)
	pathSep   = regexp.MustCompile(`/`)
)

func main() {
	opts := options{
		covermode: "set",
		maxJobs:   3,
		timeout:   10 * time.Minute,
		coverdir:  "/tmp",
		exclude:   []string{"vendor/"},
	}

	parsed, err := docopt.Parse(docstring, nil, true, "", false)
	if err != nil {
		log.Fatal(err)
	}

	err = coerce.Struct(&opts, parsed)
	if err != nil {
		log.Fatal(err)
	}

	gl := exec.Command("go", "list", opts.packageSelector)
	list, err := gl.CombinedOutput()
	if err != nil {
		fmt.Print(string(list))
		os.Exit(1)
	}
	pkgs := strings.Split(string(list), "\n")
	pkgs = pkgs[:len(pkgs)-1]

	excludeRE := regexp.MustCompile(strings.Join(opts.exclude, "|"))
	for i := 0; i < len(pkgs); {
		if excludeRE.MatchString(pkgs[i]) {
			pkgs[i] = pkgs[len(pkgs)-1]
			pkgs = pkgs[:len(pkgs)-1]
		} else {
			i++
		}
	}
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
