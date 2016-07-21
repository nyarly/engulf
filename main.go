package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

type blank struct{}
type result struct {
	j *exec.Cmd
	o []byte
	e error
}

const (
	version   = `0.1`
	docstring = `Multiple-package coverage runner for Go
Usage: engulf [options] <package-selector>
`
)

var (
	nothing   = blank{}
	pkgWarnRE = regexp.MustCompile(`(?m)^warning: no packages being tested depend on \S+$\n`)
	pathSep   = regexp.MustCompile(`/`)
)

func main() {
	var cpkg, dir, exclude string
	var jobs int
	var help bool
	flag.BoolVar(&help, "help", false, "Print this info")
	flag.IntVar(&jobs, "max-jobs", 3, "Run at most `n` test processes at once")
	flag.StringVar(&cpkg, "coverpkg", "", "Passed directly to go test - defaults to <package-selector>")
	flag.StringVar(&dir, "coverdir", "/tmp", "Storage `directory` for cover profiles - defaults to /tmp")
	flag.StringVar(&exclude, "exclude", "/vendor/", "`pattern` to exclude from coverage list")
	flag.Parse()

	if help {
		fmt.Print(docstring)
		flag.PrintDefaults()
		os.Exit(0)
	}

	sel := flag.Args()[len(flag.Args())-1]

	gl := exec.Command("go", "list", sel)
	list, err := gl.CombinedOutput()
	if err != nil {
		fmt.Print(string(list))
		os.Exit(1)
	}
	pkgs := strings.Split(string(list), "\n")
	pkgs = pkgs[:len(pkgs)-1]
	excludeRE := regexp.MustCompile(exclude)
	for i := 0; i < len(pkgs); {
		if excludeRE.MatchString(pkgs[i]) {
			pkgs[i] = pkgs[len(pkgs)-1]
			pkgs = pkgs[:len(pkgs)-1]
		} else {
			i++
		}

	}
	if cpkg == "" {
		cpkg = strings.Join(pkgs, ",")
	}

	covArgs := flag.Args()[:len(flag.Args())-1]

	covJobs := make(chan *exec.Cmd, jobs)
	startC := make(chan blank, jobs)
	stopC := make(chan result, jobs)

	go queueJobs(covJobs, dir, cpkg, covArgs, pkgs)

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

func queueJobs(covJobs chan *exec.Cmd, dir, cpkg string, covArgs, pkgs []string) {
	for _, p := range pkgs {
		path := profilepath(dir, p)
		os.MkdirAll(filepath.Dir(path), os.ModeDir|os.ModePerm)
		cov := exec.Command("go", "test", "--coverprofile="+path, "--coverpkg="+cpkg)
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
		cmd := exec.Command("go", append([]string{"test"}, j.Args[4:]...)...) //brittle
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
