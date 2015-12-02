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
	out           *bytes.Buffer
	insideBlock   bool // Block comment
	indentation   int  // Indentation level
	lastEmpty     bool
	lastComment   bool
	lastStar      bool // Block comment, last line started with a star.
	lastLabel     bool
	anyContents   bool
	lastContinued bool // Last line continued
	queued        []statement
	defines       map[string]struct{}
}

type statement struct {
	instruction string
	params      []string // Parameters
	comment     string   // Without slashes
	function    bool     // Probably define call
	continued   bool     // Multiline statement, continues on next line
}

// Add a new input line.
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
			end := strings.TrimSpace(s[:ends])
			if f.lastStar {
				end = end + " */\n"
			} else {
				end = end + "*/\n"
			}
			f.out.WriteString(end)
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
				f.lastStar = true
			} else {
				f.lastStar = false
			}
			_, err := fmt.Fprintln(f.out, s)
			return err
		}
	}

	// Comment only line.
	if strings.HasPrefix(s, "//") {
		// Non-comment content is now added.
		defer func() {
			f.anyContents = true
			f.lastEmpty = false
			f.lastStar = false
		}()

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

		// Preserve whitespace if the first character after the comment
		// is a whitespace
		ts := strings.TrimSpace(s)
		if (ts != s && len(ts) > 0) || (len(s) > 0 && strings.ContainsAny(string(s[0]), `+/`)) {
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

	if strings.Contains(s, "/*") && !strings.HasSuffix(s, `\`) {
		starts := strings.Index(s, "/*")
		ends := strings.Index(s, "*/")
		pre := s[:starts]
		pre = strings.TrimSpace(pre)
		if len(pre) > 0 {
			if strings.HasSuffix(s, `\`) {
				goto exitcomm
			}
			// Add items before the comment section as a line.
			if ends > starts && ends >= len(s)-2 {
				comm := strings.TrimSpace(s[starts+2 : ends])
				return f.addLine([]byte(pre + " //" + comm))
			}
			err := f.addLine([]byte(pre))
			if err != nil {
				return err
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
			s = strings.TrimSpace(strings.TrimSuffix(s, `\`)) + ` \`
		}

		// Otherwise output
		fmt.Fprint(f.out, "/*")
		s = strings.TrimSpace(s[starts+2:])
		f.insideBlock = ends < 0
		f.lastComment = true
		f.lastStar = true
		if len(s) == 0 {
			f.out.WriteByte('\n')
			return nil
		}
		f.out.WriteByte(' ')
		f.out.WriteString(s + "\n")
		return nil
	}
exitcomm:

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
		if f.lastContinued {
			f.indentation = 0
			f.lastContinued = false
		}
		f.lastEmpty = true
		return f.out.WriteByte('\n')
	}

	// Non-comment content is now added.
	defer func() {
		f.anyContents = true
		f.lastEmpty = false
		f.lastStar = false
	}()

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

	// Move anything that isn't a comment to the next line
	if st.isLabel() && len(st.params) > 0 && !st.continued {
		idx := strings.Index(s, ":")
		st = newStatement(s[:idx+1], f.defines)
		defer f.addLine([]byte(s[idx+1:]))
	}

	// Should this line be at level 0?
	if st.level0() && !(st.continued && f.lastContinued) {
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
	if st.isTerminator() || (f.lastContinued && !st.continued) {
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
	f.lastContinued = st.continued
	return nil
}

// indent the current line with current indentation.
func (f *fstate) indent() error {
	for i := 0; i < f.indentation; i++ {
		err := f.out.WriteByte('\t')
		if err != nil {
			return err
		}
	}
	return nil
}

// flush any queued commands
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

// newStatement will parse a line and return it as a statement.
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
		s = strings.TrimSpace(s[:startcom])
	}

	// Handle defined macro calls
	if len(defs) > 0 {
		inst := strings.Split(st.instruction, "(")[0]
		if _, ok := defs[inst]; ok {
			st.function = true
		}
	}
	if strings.HasPrefix(s, "/*") {
		st.function = true
	}
	// We may not have it defined as a macro, if defined in an external
	// .h file, so we try to detect the remaining ones.
	if strings.ContainsAny(st.instruction, "(_") {
		st.function = true
	}
	if len(st.params) > 0 && strings.HasPrefix(st.params[0], "(") {
		st.function = true
	}
	if st.function {
		st.instruction = s
	}

	s = strings.TrimPrefix(s, st.instruction)
	st.instruction = strings.Replace(st.instruction, "\t", " ", -1)
	s = strings.TrimSpace(s)

	st.setParams(s)

	// Remove trailing ;
	if len(st.params) > 0 {
		st.params[len(st.params)-1] = strings.TrimSuffix(st.params[len(st.params)-1], ";")
	} else {
		st.instruction = strings.TrimSuffix(st.instruction, ";")
	}

	// Register line continuations.
	if len(st.params) > 0 {
		p := st.params[len(st.params)-1]
		if st.willContinue() {
			p = strings.TrimSuffix(st.params[len(st.params)-1], `\`)
			p = strings.TrimSpace(p)
			if len(p) > 0 {
				st.params[len(st.params)-1] = p
			} else {
				st.params = st.params[:len(st.params)-1]
			}
			st.continued = true
		}
	}
	if strings.HasSuffix(st.instruction, `\`) {
		i := strings.TrimSuffix(st.instruction, `\`)
		st.instruction = strings.TrimSpace(i)
		st.continued = true
	}

	return &st
}

func (st *statement) setParams(s string) {
	st.params = make([]string, 0)
	runes := []rune(s)
	start := 0
	lastSlash := false
	inComment := false
	lastAst := false
	for i, r := range runes {
		switch r {
		case ',':
			if inComment {
				lastSlash = false
				lastAst = false
				continue
			}
			c := strings.TrimSpace(string(runes[start:i]))
			if len(c) > 0 {
				st.params = append(st.params, c)
			}
			start = i + 1
		case '/':
			if lastAst && inComment {
				inComment = false
				lastSlash = false
			} else {
				lastSlash = false
				lastSlash = true
			}
		case '*':
			if lastSlash {
				inComment = true
			} else {
				lastAst = true
			}
		case '\t':
			if !st.isPreProcessor() {
				runes[i] = ' '
			}
		default:
			lastSlash = false
			lastAst = false
		}
	}
	if start < len(runes) {
		c := strings.TrimSpace(string(runes[start:]))
		if len(c) > 0 {
			st.params = append(st.params, c)
		}
	}
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

// isGlobal returns true if the current instruction is
// a global. Currently that is DATA and GLOBL
func (st statement) isGlobal() bool {
	up := strings.ToUpper(st.instruction)
	return up == "DATA" || up == "GLOBL"
}

func (st statement) isTEXT() bool {
	up := strings.ToUpper(st.instruction)
	return up == "TEXT" || up == "DATA" || up == "GLOBL"
}

// We attempt to identify "terminators", after which
// indentation is likely to be level 0.
func (st statement) isTerminator() bool {
	up := strings.ToUpper(st.instruction)
	return up == "RET" || up == "JMP"
}

// Detects commands based on case.
func (st statement) isCommand() bool {
	up := strings.ToUpper(st.instruction)
	return up == st.instruction
}

// Detect if last character is '\', indicating a multiline statement.
func (st statement) willContinue() bool {
	if st.continued {
		return true
	}
	if len(st.params) == 0 {
		return false
	}
	return strings.HasSuffix(st.params[len(st.params)-1], `\`)
}

// define returns the macro defined in this line.
// if none is defined "" is returned.
func (st statement) define() string {
	if st.instruction == "#define" && len(st.params) > 0 {
		r := strings.TrimSpace(strings.Split(st.params[0], "(")[0])
		r = strings.Trim(r, `\`)
		return r
	}
	return ""
}

// formatStatements will format a slice of statements and return each line
// as a separate string.
func formatStatements(s []statement) []string {
	res := make([]string, len(s))
	maxParam := 0
	maxInstr := 0
	maxAlone := 0
	maxComm := 0
	for _, x := range s {
		il := len([]rune(x.instruction)) + 1 // Instruction length
		l := il
		// Ignore length if we are a define "function"
		if l > maxInstr && !x.function {
			maxInstr = l
		}
		if x.function && il > maxAlone {
			maxAlone = il
		}
		if len(x.params) > 1 {
			l = 2 * (len(x.params) - 1) // Spaces between parameters
		} else {
			l = 0
		}
		// Add parameters
		for _, y := range x.params {
			l += len([]rune(y))
		}
		l++
		if l > maxParam {
			maxParam = l
		}
		// Add comment (for line continuations)
		l += len([]rune(x.comment))
		if l > maxComm {
			maxComm = l
		}
	}

	maxComm += maxInstr
	maxParam += maxInstr
	if maxAlone > maxComm {
		maxComm = maxAlone
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
			it := maxParam - len([]rune(r))
			for i := 0; i < it; i++ {
				r = r + " "
			}
			r += fmt.Sprintf("// %s", x.comment)
		}
		if x.continued {
			it := maxComm - len([]rune(r))
			for i := 0; i < it; i++ {
				r = r + " "
			}
			r += `\`
		}
		res[i] = r
	}
	return res
}
