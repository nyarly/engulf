package main

import (
	"log"
	"time"

	"github.com/docopt/docopt-go"
	"github.com/nyarly/coerce"
)

type options struct {
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
	excludeFiles    string
	mergeBase       string
	onlyMerge       bool
}

func defaultOpts() options {
	return options{
		coverdir: "/tmp",
	}
}

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
	-x=<pattern>, --exclude=<pattern>       comma separated package <patterns> to exclude from coverage list [default: vendor/]
	--exclude-files=<pattern>               comma separated file <patterns> to exclude from coverage list
	--merge-base=<filename>                 base name to use for merging coverage
	--only-merge                            Don't do coverage, just merge coverage results
`
)

func parseOpts() options {
	opts := defaultOpts()

	parsed, err := docopt.Parse(docstring, nil, true, version, false, false)
	if err != nil {
		log.Fatal("parse:", err)
	}

	err = coerce.Struct(&opts, parsed, "-%s", "--%s", "<%s>")
	if err != nil {
		log.Fatal("coerce: ", err)
	}

	return opts
}
