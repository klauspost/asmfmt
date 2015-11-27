package asmfmt

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strings"
)

// Format the input and return the formatted data.
// If any error is encountered, no data will be returned.
func Format(in io.Reader) ([]byte, error) {
	var src *bufio.Reader
	var ok bool
	src, ok = in.(*bufio.Reader)
	if !ok {
		src = bufio.NewReader(in)
	}
	dst := &bytes.Buffer{}
	state := fstate{out: dst}
	for {
		data, _, err := src.ReadLine()
		if err == io.EOF {
			err := state.flush()
			if err != nil {
				return nil, err
			}
			break
		}
		if err != nil {
			return nil, err
		}
		err = state.addLine(data)
		if err != nil {
			return nil, err
		}
	}
	return dst.Bytes(), nil
}

type fstate struct {
	out            *bytes.Buffer
	insideBlock    bool // Block comment
	indentation    int  // Indentation level
	lastEmpty      bool
	lastComment    bool
	lastLabel      bool
	anyContents    bool
	inContinuation bool // Inside a multiline statement
	queued         []statement
}

type statement struct {
	instruction string
	params      []string
	comment     string // Without slashes
	function    bool   // Probably define call
}

func (f *fstate) addLine(b []byte) error {
	s := string(b)
	s = strings.TrimSpace(s)
	// Inside block comment
	if f.insideBlock {
		defer func() {
			f.lastComment = true
		}()
		if strings.Contains(s, "*/") {
			ends := strings.Index(s, "*/")
			end := s[:ends]
			f.out.WriteString(end + " */\n")
			f.insideBlock = false
			s = strings.TrimSpace(s[ends+2:])
			if len(s) == 0 {
				return nil
			}
		} else {
			// Insert a space on lines that begin with '*'
			if strings.HasPrefix(s, "*") {
				err := f.out.WriteByte(' ')
				if err != nil {
					return err
				}
			}
			_, err := fmt.Fprintln(f.out, s)
			return err
		}
	}
	if strings.Contains(s, "/*") {
		starts := strings.Index(s, "/*")
		ends := strings.Index(s, "*/")
		pre := s[:starts]
		pre = strings.TrimSpace(pre)
		if len(pre) > 0 {
			err := f.addLine([]byte(pre))
			if err != nil {
				return err
			}
			// Convert end-of-line /* comment */ to // comment
			if ends > starts && ends >= len(s)-2 {
				f.queued[len(f.queued)-1].comment += s[starts+2:ends]
				ends = 0 
				return nil
			}
		}

		err := f.flush()
		if err != nil {
			return err
		}

		// Convert single line /* comment */ to // Comment
		if ends > starts && ends >= len(s)-2 {
			return f.addLine([]byte("//" + s[starts+2:ends]))
		}

		if strings.HasSuffix(s, `\`) {
			f.indent()
		}
		// Otherwises output
		fmt.Fprint(f.out, "/*")
		s = strings.TrimSpace(s[starts+2:])
		f.insideBlock = ends < 0
		f.lastComment = true
		if len(s) == 0 {
			f.out.WriteByte('\n')
			return nil
		}
		f.out.WriteByte(' ')
		f.out.WriteString(s + "\n")
		return nil
	}

	defer func() {
		f.anyContents = true
	}()
	if len(s) == 0 {
		err := f.flush()
		if err != nil {
			return err
		}
		// No more than two empty lines in a row
		// cannot start with NL
		if f.lastEmpty || !f.anyContents {
			return nil
		}
		f.lastEmpty = true
		return f.out.WriteByte('\n')
	}

	defer func() {
		f.lastEmpty = false
	}()

	// Comment only line.
	if strings.HasPrefix(s, "//") {
		s = strings.TrimPrefix(s, "//")
		if strings.HasPrefix(s, " ") {
			s = s[1:]
		}

		err := f.flush()
		if err != nil {
			return err
		}

		err = f.newLine()
		if err != nil {
			return err
		}

		err = f.indent()
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(f.out, "//", s)
		if err != nil {
			return err
		}
		f.lastComment = true
		return nil
	}

	defer func() {
		f.lastComment = false
	}()
	st := newStatement(s)
	if st == nil {
		return nil
	}

	if st.level0() {
		err := f.flush()
		if err != nil {
			return err
		}

		// Add newline before jump target.
		err = f.newLine()
		if err != nil {
			return err
		}

		f.indentation = 0
		f.queued = append(f.queued, *st)
		err = f.flush()
		if err != nil {
			return err
		}
		if !st.isPreProcessor()  && !st.isGlobal() {
			f.indentation = 1
		}
		f.lastLabel = true
		return nil
	}

	defer func() {
		f.lastLabel = false
	}()
	f.queued = append(f.queued, *st)
	if st.isTerminator() {
		err := f.flush()
		if err != nil {
			return err
		}
		f.indentation = 0
	} else if st.isCommand() {
		// handles cases where a JMP/RET isn't a terminator
		f.indentation = 1
	}
	return nil
}

func (f *fstate) indent() error {
	for i := 0; i < f.indentation; i++ {
		err := f.out.WriteByte('\t')
		if err != nil {
			return err
		}
	}
	return nil
}

func (f *fstate) flush() error {
	s := formatStatements(f.queued)
	for _, line := range s {
		err := f.indent()
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(f.out, line)
		if err != nil {
			return err
		}
	}
	f.queued = nil
	return nil
}

// Add a newline, unless last line was empty or a comment
func (f *fstate) newLine() error {
	// Always newline before comment-only line.
	if !f.lastEmpty && !f.lastComment && !f.lastLabel && f.anyContents {
		return f.out.WriteByte('\n')
	}
	return nil
}

func newStatement(s string) *statement {
	s = strings.TrimSpace(s)

	st := statement{}
	fields := strings.Fields(s)
	if len(fields) < 1 {
		return nil
	}
	st.instruction = fields[0]

	// Fix where a comment start if any
	startcom := strings.Index(s, "//")
	if startcom > 0 {
		st.comment = strings.TrimSpace(s[startcom+2:])
		s = s[:startcom]
	}

	// Handle define "function" calls
	if strings.Contains(st.instruction, "(") {
		st.instruction = s
		st.function = true
	}

	s = strings.TrimPrefix(s, st.instruction)
	s = strings.TrimSpace(s)

	// Split parameters
	fields = strings.Split(s, ",")
	st.params = make([]string, 0, len(fields))
	for i := range fields {
		field := strings.TrimSpace(fields[i])
		if len(field) > 0 {
			st.params = append(st.params, field)
		}
	}
	if len(st.params) > 0 {
		st.params[len(st.params)-1] = strings.TrimSuffix(st.params[len(st.params)-1], ";")
	}
	return &st
}

func (st statement) level0() bool {
	return st.isLabel() || st.isTEXT() || st.isPreProcessor()
}

func (st statement) isLabel() bool {
	return strings.HasSuffix(st.instruction, ":")
}

func (st statement) isPreProcessor() bool {
	return strings.HasPrefix(st.instruction, "#")
}

func (st statement) isGlobal() bool {
	up := strings.ToUpper(st.instruction)
	return up == "DATA" || up == "GLOBL"	
}

func (st statement) isTEXT() bool {
	up := strings.ToUpper(st.instruction)
	return up == "TEXT" || up == "DATA" || up == "GLOBL"
}

func (st statement) isTerminator() bool {
	up := strings.ToUpper(st.instruction)
	return up == "RET" || up == "JMP"
}

func (st statement) isCommand() bool {
	up := strings.ToUpper(st.instruction)
	return up == st.instruction
}

func (st statement) willContinue() bool {
	if len(st.params) == 0 {
		return false
	}
	return strings.HasSuffix(st.params[len(st.params)-1], `\`)
}

func formatStatements(s []statement) []string {
	res := make([]string, len(s))
	maxParam := 0
	maxInstr := 0
	for _, x := range s {
		l := len([]rune(x.instruction)) + 1 // Instruction length
		// Ignore length if we are a define "function"
		if l > maxInstr && !x.function{
			maxInstr = l
		}
		l = len(x.params) // Spaces between parameters
		// Add parameters
		for _, y := range x.params {
			l += len([]rune(y))
		}
		l++
		if l > maxParam {
			maxParam = l
		}
	}

	for i, x := range s {
		p := strings.Join(x.params, ", ")
		r := x.instruction
		if len(x.params) > 0 || len(x.comment) > 0 {
			for len(r) < maxInstr {
				r += " "
			}
		}
		r = r + p
		if len(x.comment) > 0 {
			it := maxParam - len([]rune(r)) + maxInstr
			for i := 0; i < it; i++ {
				r = r + " "
			}
			r += fmt.Sprintf("// %s", x.comment)
		}
		res[i] = r
	}
	return res
}
