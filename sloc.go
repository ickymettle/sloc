package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"regexp"
	"text/tabwriter"
)

type Commenter struct {
	LineComment  string
	StartComment string
	EndComment   string
	Nesting      bool
}

var (
	noComments = Commenter{"\000", "\000", "\000", false}
	cComments  = Commenter{`//`, `/*`, `*/`, false}
	shComments = Commenter{`#`, "\000", "\000", false}
)

type Language struct {
	Namer
	Matcher
	Commenter
}

var (
	blankR = regexp.MustCompile(`^[ \t]*$`)
)

func (l Language) Update(c []byte, s *Stats) {
	s.FileCount++
	// line pointers
	lStart := 0

	inComment := 0 // this is an int for nesting
	inLComment := false
	lc := []byte(l.LineComment)
	sc := []byte(l.StartComment)
	ec := []byte(l.EndComment)
	lp, sp, ep := 0, 0, 0

	for i, b := range c {
		if b == lc[lp] && !(inComment > 0) {
			lp++
			if lp == len(lc) {
				inLComment = true
				lp = 0
			}
		} else { lp = 0 }
		if b == sc[sp] && !inLComment {
			sp++
			if sp == len(sc) {
				inComment++
				if inComment > 1 && !l.Nesting {
					inComment = 1
				}
				sp = 0
			}
		} else { sp = 0 }
		if b == ec[ep] && !inLComment && inComment > 0 {
			ep++
			if ep == len(ec) {
				inComment--
				ep = 0
			}
		} else { ep = 0 }

		// Note that lines with both code and comment count towards
		// each, but are not counted twice in the total.
		if b == byte('\n') {
			s.TotalLines++
			if blankR.Match(c[lStart:i]) {
				s.BlankLines++
			}
			if inComment > 0 || inLComment {
				if !blankR.Match(c[lStart:i]) {
					s.CodeLines++
				}
				inLComment = false
				s.CommentLines++
			} else { s.CodeLines++ }
			lStart = i + 1
			continue
		}
	}
}

type Namer string

func (l Namer) Name() string { return string(l) }

type Matcher func(string) bool

func (m Matcher) Match(fname string) bool { return m(fname) }

func mExt(exts ...string) Matcher {
	return func(fname string) bool {
		for _, ext := range exts {
			if ext == path.Ext(fname) {
				return true
			}
		}
		return false
	}
}

func mName(names ...string) Matcher {
	return func(fname string) bool {
		for _, name := range names {
			if name == path.Base(fname) {
				return true
			}
		}
		return false
	}
}

type Stats struct {
	FileCount    int
	TotalLines   int
	CodeLines    int
	BlankLines   int
	CommentLines int
}

var info = map[string]*Stats{}

var languages = []Language{
	Language{"C", mExt(".c", ".h"), cComments},
	Language{"C++", mExt(".cc", ".cpp", ".cxx", ".hh", ".hpp", ".hxx"), cComments},
	Language{"Go", mExt(".go"), cComments},
	Language{"Haskell", mExt(".hs", ".lhs"), noComments},
	Language{"Perl", mExt(".pl", ".pm"), shComments},
	Language{"Python", mExt(".py"), noComments},
	Language{"Lisp", mExt(".lsp"), noComments},
	Language{"Make", mName("makefile", "Makefile", "MAKEFILE"), shComments},
	Language{"HTML", mExt(".htm", ".html", ".xhtml"), noComments},
}

func handleFile(fname, content string) {
	var l Language
	ok := false
	for _, lang := range languages {
		if lang.Match(fname) {
			ok = true
			l = lang
			break
		}
	}
	if !ok {
		return // ignore this file
	}
	i, ok := info[l.Name()]
	if !ok {
		i = &Stats{}
		info[l.Name()] = i
	}
	c, err := ioutil.ReadFile(fname)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  ! %s\n", fname)
		return
	}
	l.Update(c, i)
}

func printInfo() {
	w := tabwriter.NewWriter(os.Stdout, 2, 8, 2, ' ', tabwriter.AlignRight)
	fmt.Fprintln(w, "Language\tFiles\tCode\tComment\tBlank\tTotal\t")
	for n, i := range info {
		fmt.Fprintf(
			w,
			"%s\t%d\t%d\t%d\t%d\t%d\t\n",
			n,
			i.FileCount,
			i.CodeLines,
			i.CommentLines,
			i.BlankLines,
			i.TotalLines)
	}
	w.Flush()
}

var files []string

func add(n string) {
	fi, err := os.Stat(n)
	if err != nil {
		goto invalid
	}
	if fi.IsDir() {
		fs, err := ioutil.ReadDir(n)
		if err != nil {
			goto invalid
		}
		for _, f := range fs {
			if f.Name()[0] != '.' {
				add(path.Join(n, f.Name()))
			}
		}
		return
	}
	if fi.Mode()&os.ModeType == 0 {
		files = append(files, n)
		return
	}

	println(fi.Mode())

invalid:
	fmt.Fprintf(os.Stderr, "  ! %s\n", n)
}

func main() {
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		args = append(args, `.`)
	}

	for _, n := range args {
		add(n)
	}

	for _, f := range files {
		c, err := ioutil.ReadFile(f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  ! %s\n", f)
		}
		cs := string(c)
		handleFile(f, cs)
	}
	printInfo()
}
