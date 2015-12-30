package asmfmt

import (
	"bytes"
	"flag"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "update .golden files")

func init() {
	flag.Parse()
}

func runTest(t *testing.T, in, out string) {
	f, err := os.Open(in)
	if err != nil {
		t.Error(err)
		return
	}
	defer f.Close()

	got, err := Format(f)
	if err != nil {
		t.Error(in, "-", err)
		return
	}

	expected, err := ioutil.ReadFile(out)
	if err != nil && !*update {
		t.Error(out, "-", err)
		return
	}

	// Convert expected file to LF in case someone did it for us.
	expected = []byte(strings.Replace(string(expected), "\r\n", "\n", -1))

	if !bytes.Equal(got, expected) {
		if *update {
			if in != out {
				if err := ioutil.WriteFile(out, got, 0666); err != nil {
					t.Error(err)
				}
				return
			}
			// in == out: don't accidentally destroy input
			t.Errorf("WARNING: -update did not rewrite input file %s", in)
		}

		t.Errorf("(gofmt %s) != %s (see %s.asmfmt)", in, out, in)
		d, err := diff(expected, got)
		if err == nil {
			t.Errorf("%s", d)
		}
		if err := ioutil.WriteFile(in+".asmfmt", got, 0666); err != nil {
			t.Error(err)
		}
	}
}

// TestRewrite processes testdata/*.input files and compares them to the
// corresponding testdata/*.golden files. The gofmt flags used to process
// a file must be provided via a comment of the form
//
//	//gofmt flags
//
// in the processed file within the first 20 lines, if any.
func TestRewrite(t *testing.T) {
	// determine input files
	match, err := filepath.Glob("testdata/*.in")
	if err != nil {
		t.Fatal(err)
	}

	for _, in := range match {
		out := in // for files where input and output are identical
		if strings.HasSuffix(in, ".in") {
			out = in[:len(in)-len(".in")] + ".golden"
		}
		runTest(t, in, out)
		if in != out {
			// Check idempotence.
			runTest(t, out, out)
		}
	}
}

func diff(b1, b2 []byte) (data []byte, err error) {
	f1, err := ioutil.TempFile("", "asmfmt")
	if err != nil {
		return
	}
	defer os.Remove(f1.Name())
	defer f1.Close()

	f2, err := ioutil.TempFile("", "asmfmt")
	if err != nil {
		return
	}
	defer os.Remove(f2.Name())
	defer f2.Close()

	f1.Write(b1)
	f2.Write(b2)

	data, err = exec.Command("diff", "-u", f1.Name(), f2.Name()).CombinedOutput()
	if len(data) > 0 {
		// diff exits with a non-zero status when the files don't match.
		// Ignore that failure as long as we get output.
		err = nil
	}
	return

}

// Go files must fail.
func TestGoFile(t *testing.T) {
	input := `package main

	func main() {
	}
	`
	_, err := Format(bytes.NewBuffer([]byte(input)))
	if err == nil {
		t.Error("go file not detected")
		return
	}
}

// Files containg zero byte values must fail.
func TestZeroByteFile(t *testing.T) {
	var input = []byte{13, 13, 10, 0, 0, 0, 13}
	_, err := Format(bytes.NewBuffer(input))
	if err == nil {
		t.Fatal("file containing zero (0) byte values not rejected")
		return
	}
}
