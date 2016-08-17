package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/tools/cover"
)

type (
	blank  struct{}
	result struct {
		j *exec.Cmd
		o []byte
		e error
	}
)

var (
	nothing   = blank{}
	pkgWarnRE = regexp.MustCompile(`(?m)^warning: no packages being tested depend on \S+$\n`)
	pathSep   = regexp.MustCompile(`/`)
)

func main() {
	opts := parseOpts()

	gl := exec.Command("go", "list", opts.packageSelector)
	list, err := gl.CombinedOutput()
	if err != nil {
		fmt.Print(string(list))
		os.Exit(1)
	}
	pkgs := strings.Split(string(list), "\n")
	pkgs = pkgs[:len(pkgs)-1]

	excludeRE := regexp.MustCompile(strings.Join(strings.Split(opts.exclude, ","), "|"))
	excludeFilesRE := regexp.MustCompile(strings.Join(strings.Split(opts.excludeFiles, ","), "|"))
	for i := 0; i < len(pkgs); {
		if excludeRE.MatchString(pkgs[i]) {
			pkgs[i] = pkgs[len(pkgs)-1]
			pkgs = pkgs[:len(pkgs)-1]
		} else {
			i++
		}
	}

	fmt.Printf("Running with: %s\n", strings.Join(pkgs, ","))

	if !opts.onlyMerge {
		if opts.coverpkg == "" {
			opts.coverpkg = strings.Join(pkgs, ",")
		}
		fmt.Printf("Covering:     %s\n", opts.coverpkg)

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
		mergedProfiles(opts.coverdir, opts.mergeBase, pkgs, excludeFilesRE)
	}
}

func mergedProfiles(dir, basename string, pkgs []string, excludeFilesRE *regexp.Regexp) {
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
				filed = &cover.Profile{
					FileName: prof.FileName,
					Mode:     prof.Mode,
					Blocks:   prof.Blocks,
				}
				moded[prof.FileName] = filed
			} else {
				big := len(prof.Blocks)
				sml := len(filed.Blocks)
				newList := mergeBlocks(filed.Blocks, prof.Blocks)
				if big < sml {
					big, sml = sml, big
				}
				// XXX This is too low on purpose
				if len(newList) > big+3*sml {
					log.Println("Existing:")
					for _, b := range filed.Blocks {
						log.Println(b)
					}
					log.Println("Incoming:")
					for _, b := range prof.Blocks {
						log.Println(b)
					}
					log.Println("Result:")
					for _, b := range newList {
						log.Println(b)
					}
					log.Panicf("Resulting blocklist is way too big: %d > %d (=%d + 3* %d)", len(newList), big+sml*3, big, sml)
				}
				filed.Blocks = newList
			}
		}
	}

	for kind, m := range merged {
		var list []*cover.Profile
		for _, prf := range m {
			list = append(list, prf)
		}

		fname := filepath.Join(dir, kind+basename)
		writeCoverprofile(fname, kind, list, excludeFilesRE)
	}
}

func writeCoverprofile(filename, mode string, list []*cover.Profile, excludeFilesRE *regexp.Regexp) error {
	pf, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer pf.Close()

	fmt.Fprintf(pf, "mode: %s\n", mode)

	for _, p := range list {
		for _, b := range p.Blocks {
			if !excludeFilesRE.MatchString(p.FileName) {
				fmt.Fprintf(pf, "%s:%d.%d,%d.%d %d %d\n",
					p.FileName,
					b.StartLine, b.StartCol, b.EndLine, b.EndCol,
					b.NumStmt, b.Count,
				)
			}
		}
	}
	return nil
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
