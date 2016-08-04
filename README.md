# Engulf

Coverage for multiple packages.

## Install

`go get github.com/nyarly/engulf`

## Usage

`engulf --coverdir=/tmp/proj ./...`

Rejoice. There are coverage files in /tmp/proj now.

## Features

Engulf enumerates packages and runs all the tests in them,
like e.g. `go test ./...`.
What it does that's special is that it collects code coverage
as it runs the tests for each package
for all the packages it's running tests for.

What makes it really special is that if you run
`engulf ./... --coverdir=/tmp/prof --merge-base=merged.txt`
it will also merge the resulting profiles.
It does this by actually shuffling the profiles together,
as opposed to the usual process
that involves concatenating coverage files.

(It's my understanding that concatentated files will
mis-report coverage
because only the first list of coverage blocks per file will matter.)
