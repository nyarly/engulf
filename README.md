# Engulf

Coverage for multiple packages.

## Install

`go get github.com/nyarly/engulf`

## Usage

`engulf -coverdir /tmp/proj ./...`

Rejoice. There are coverage files in /tmp/proj now.

Also notice that engulf adds a `BEGIN` line as it starts testing each package.
I wanted to be able to use Vim's errorfmt.
