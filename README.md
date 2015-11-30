# asmfmt
Go Assembler Formatter

This will format your assembler code in a similar way that `gofmt` formats your Go code.

# install

To install the standalone formatter, 
`go get -u github.com/klauspost/asmfmt/cmd/asmfmt`

There are also replacements for `gofmt`, `goimports` and `goreturns`, which will process `.s` files alongside your go files when formatting a package.

Toy can choose which to install:
```
go get -u github.com/klauspost/asmfmt/cmd/gofmt/...
go get -u github.com/klauspost/asmfmt/cmd/goimports/...
go get -u github.com/klauspost/asmfmt/cmd/goreturns/...
```

# usage

`asmfmt [flags] [path ...]`

The flags are similar to `gofmt`:
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

# formatting

* It uses tabs for indentation and blanks for alignment.
* It will remove trailing whitespace.
* It will align the first parameter.
* It will align all comments in a block.
* It will eliminate multiple black lines.
* It will convert single line block comments to line comments.
* Automatic indentation.
* Line comments have a space after `//`.
* There is always a space between paramters.
* Macros are tracked.
* `TEXT`, `DATA` and `GLOBL` and labels are level 0 indentation.

TODO:
* Align `\` in multiline macros.
