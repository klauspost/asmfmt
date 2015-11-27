package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/klauspost/asmfmt"
)

func main() {
	b, err := ioutil.ReadFile(os.Args[1])
	if err != nil {
		panic(err)
	}

	res, err := asmfmt.Format(bytes.NewBuffer(b))
	if err != nil {
		panic(err)
	}
	fmt.Println(string(res))
}
