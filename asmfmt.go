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
	state := fstate{out: dst, defines: make(map[string]struct{})}
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
	defines        map[string]struct{}
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
			q := len(f.queued)
			// Add items before the comment section as a line.
			err := f.addLine([]byte(pre))
			if err != nil {
				return err
			}
			// If new instruction was added.
			if len(f.queued) > q {
				// Convert end-of-line /* comment */ to // comment
				if ends > starts && ends >= len(s)-2 {
					f.queued[len(f.queued)-1].comment += strings.TrimSpace(s[starts+2 : ends])
					ends = 0
					return nil
				}
			}
		}

		err := f.flush()
		if err != nil {
			return err
		}

		// Convert single line /* comment */ to // Comment
		if ends > starts && ends >= len(s)-2 {
			return f.addLine([]byte("// " + strings.TrimSpace(s[starts+2:ends])))
		}

		// Comments inside multiline defines.
		if strings.HasSuffix(s, `\`) {
			f.indent()
		}

		// Otherwise output
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

	// Non-comment content is now added.
	defer func() {
		f.anyContents = true
		f.lastEmpty = false
	}()

	// Comment only line.
	if strings.HasPrefix(s, "//") {
		s = strings.TrimPrefix(s, "//")

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

		// Preserve whitespace if the first after the comment
		// is a whitespace
		ts := strings.TrimSpace(s)
		if (ts != s && len(ts) > 0) || strings.HasPrefix(s, "+") {
			_, err = fmt.Fprintln(f.out, "//"+s)
		} else if len(ts) > 0 {
			// Insert a space before the comment
			_, err = fmt.Fprintln(f.out, "//", s)
		} else {
			_, err = fmt.Fprintln(f.out, "//")
		}
		if err != nil {
			return err
		}
		f.lastComment = true
		return nil
	}

	defer func() {
		f.lastComment = false
	}()

	st := newStatement(s, f.defines)
	if st == nil {
		return nil
	}
	if def := st.define(); def != "" {
		f.defines[def] = struct{}{}
	}
	if st.instruction == "package" {
		if _, ok := f.defines["package"]; !ok {
			return fmt.Errorf("package instruction found. Go files are not supported")
		}
	}
	// Should this line be at level 0?
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
		if !st.isPreProcessor() && !st.isGlobal() {
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
		// Terminators should always be at level 1
		f.indentation = 1
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

func newStatement(s string, defs map[string]struct{}) *statement {
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

	// Handle defined macro calls
	if len(defs) > 0 {
		inst := strings.Split(st.instruction, "(")[0]
		if _, ok := defs[inst]; ok {
			st.function = true
		}
	}
	// We may not have it defined, if defined in an external
	// .h file, so try to detect the remaining ones.
	if strings.Contains(st.instruction, "(") {
		st.function = true
	}
	if len(st.params) > 0 && strings.HasPrefix(st.params[0], "(") {
		st.function = true
	}
	if st.function {
		st.instruction = s
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
	// Remove trailing ;
	if len(st.params) > 0 {
		st.params[len(st.params)-1] = strings.TrimSuffix(st.params[len(st.params)-1], ";")
	}
	return &st
}

// Return true if this line should be at indentation level 0.
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

func (st statement) define() string {
	if st.instruction == "#define" && len(st.params) > 0 {
		r := strings.TrimSpace(strings.Split(st.params[0], "(")[0])
		r = strings.Trim(r, `\`)
		return r
	}
	return ""
}

func formatStatements(s []statement) []string {
	res := make([]string, len(s))
	maxParam := 0
	maxInstr := 0
	for _, x := range s {
		l := len([]rune(x.instruction)) + 1 // Instruction length
		// Ignore length if we are a define "function"
		if l > maxInstr && !x.function {
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
