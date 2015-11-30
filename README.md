# asmfmt
Go Assembler Formatter

This will format your assembler code in a similar way that `gofmt` formats your Go code.

[![Build Status](https://travis-ci.org/klauspost/asmfmt.svg?branch=master)](https://travis-ci.org/klauspost/asmfmt)
[![Windows Build](https://ci.appveyor.com/api/projects/status/s729ayhkqkjf0ye6/branch/master?svg=true)](https://ci.appveyor.com/project/klauspost/asmfmt/branch/master)
[![GoDoc][1]][2]

[1]: https://godoc.org/github.com/klauspost/asmfmt?status.svg
[2]: https://godoc.org/github.com/klauspost/asmfmt

See [Example](http://www.diff-online.com/view/565c48ccabd81).

Status: Beta. The format will still change, and there may unknown cases where the formatting mangles the output. Please report any feedback in the issue section.

# install

To install the standalone formatter,
`go get -u github.com/klauspost/asmfmt/cmd/asmfmt`

There are also replacements for `gofmt`, `goimports` and `goreturns`, which will process `.s` files alongside your go files when formatting a package.

You can choose which to install:
```
go get -u github.com/klauspost/asmfmt/cmd/gofmt/...
go get -u github.com/klauspost/asmfmt/cmd/goimports/...
go get -u github.com/klauspost/asmfmt/cmd/goreturns/...
```

Using `gofmt -w mypackage` will Gofmt your Go files and format all assembler files as well.


# usage

`asmfmt [flags] [path ...]`

The flags are similar to `gofmt`, except it will only process `.s` files:
```
	-d
		Do not print reformatted sources to standard output.
		If a file's formatting is different than asmfmt's, print diffs
		to standard output.
	-e
		Print all (including spurious) errors.
	-l
		Do not print reformatted sources to standard output.
		If a file's formatting is different from asmfmt's, print its name
		to standard output.
	-w
		Do not print reformatted sources to standard output.
		If a file's formatting is different from asmfmt's, overwrite it
		with asmfmt's version.
```
You should only run `asmfmt` on files that are assembler files. Assembler files cannot be positively identified, so it will mangle non-assembler files.

# formatting

* Automatic indentation.
* It uses tabs for indentation and blanks for alignment.
* It will remove trailing whitespace.
* It will align the first parameter.
* It will align all comments in a block.
* It will eliminate multiple black lines.
* Forced newline before comments, except when preceeded by label or another comment.
* Forced newline before labels, except when preceeded by comment.
* Retains block breaks (newline between blocks).
* It will convert single line block comments to line comments.
* Line comments have a space after `//`, except if comment starts with `+`.
* There is always a space between paramters.
* Macros in the same file are tracked, and not included in parameter indentation.
* `TEXT`, `DATA` and `GLOBL` and labels are level 0 indentation.

TODO:
* Align `\` in multiline macros.
