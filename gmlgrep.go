package main

import (
    "fmt"
    "os"
    "io"
    "bytes"
    "errors"
    "bufio"
    "sync"
    //"regexp"
    "strings"
    goopt "github.com/droundy/goopt"
)
var Usage = "gmlgrep [OPTIONS...] PATTERN[...] [--] [FILES...]"
var Summary = `
  grep(1) like tool, but "record-oriented", instead of line-oriented.
  Useful to search/print multi-line log entries separated by e.g., empty
  lines, '----' or timestamps, etc. If an argument in argument list is a
  name of existing file or '-' (means stdin), such argument and all arguments
  after that will be treated as filenames to read from. Otherwise arguments
  are considered to be regex to search. (could be confusing if you specify
  nonexistent filename!)`

// The Flag function creates a boolean flag, possibly with a negating
// alternative.  Note that you can specify either long or short flags
// naturally in the same list.
var optCount = goopt.Flag([]string{"-c", "--count"}, nil,
    "Print number of matches. (same as grep -c)", "")
var optIgnoreCase = goopt.Flag([]string{"-i", "--ignore-case"}, nil,
    "Case insensitive matching. Default is case sensitive.", "")
var optInvert = goopt.Flag([]string{"-v", "--invert"}, nil,
    "Select non-matching records (same as grep -v).", "")
var optAnd   = goopt.Flag([]string{"-a", "--and"}, nil,
    "Extract records with all of patterns. (default: any)", "")
var optTimestamp  = goopt.Flag([]string{"-t", "--timestamp"}, nil,
    "Same as --rs=TIMESTAMP_REGEX, where the regex matches timestamps often used in log files, e.g., '2014-12-31 12:34:56' or 'Dec 31 12:34:56'.", "")
var optColor = goopt.Flag([]string{"--color", "--hl"}, nil,
    "Highlight matches. Default is enabled iff stdout is a TTY.", "")

const RS_REGEX = "^$|^(=====*|-----*)$"
var rs = goopt.StringWithLabel([]string{"-r", "--rs"}, RS_REGEX, "RS_REGEX",
    fmt.Sprintf("Input record separator. default: /%s/", RS_REGEX))

//////////////////////////////////////////////////////////////////////////////
//maxBufferSize = 1 * 1024 * 1024



//////////////////////////////////////////////////////////////////////////////

func checkError(e error) {
    if e != nil {
        fmt.Fprintf(os.Stdout, "ERROR: %d\n", e)
        os.Exit(1)
    }
}

func debug(format string, args ...interface{}) {
    fmt.Fprintf(os.Stderr, ">> DEBUG: " + format, args...)
}

//escape newline for debug output.
//func esc(s string) {
//    strings.Replace(s, "\n", "\\n", -1)
//}

func esc(b []byte) string {
    return strings.Replace(string(b), "\n", "\\n", -1)
}

//////////////////////////////////////////////////////////////////////////////
// Find-pattern-first algorithm

type PatternFirstFinder struct {
    found bool;
    patFinder   func(data []byte) (int, int)
    rsFinder    func(data []byte) (int, int)
    rsRevFinder func(data []byte) (int, int)
}

func NewPatternFirstFinder(pat, rs string) *PatternFirstFinder{
    //compile regex and set MLRFinder fields
    s := new(PatternFirstFinder)
    s.found = false
    s.patFinder   = func(d []byte) (int, int) { return bytes.Index(d, []byte(pat)), len(pat) }
    s.rsFinder    = func(d []byte) (int, int) { return bytes.Index(d, []byte(rs)), len(rs) }
    s.rsRevFinder = func(d []byte) (int, int) {
        if len(d) < len (rs) {
            return -1, 0
        }
        for pos := 0; pos < len(d); pos++ {
            if bytes.HasSuffix(d[0:len(d)-pos], []byte(rs)) {
                return pos, len(rs)
            }
        }
        return -1, 0
    }
    return s
}

func (s *PatternFirstFinder) Split(data []byte, atEOF bool, tooLong bool) (advance int, token []byte, err error) {
    s.found = false
    debug("split(\"%s\", %v, %v)\n", esc(data[:60]), atEOF, tooLong)

    if atEOF && len(data) == 0 {
        return 0, nil, nil
    }

    if (tooLong) {
        // so this is retry with tooLong flag enabled; we cannot request more data
        // and there's no match of the pattern in data. So we return
        rsPos, rsSize := s.rsRevFinder(data)
        if rsPos < 0 {
            return 0, nil, errors.New("record is too long and didn't fit into a buffer")
        } else {
            //return non-match records with s.found set to false
            return len(data) - rsPos + rsSize, data[len(data):len(data)], nil
        }
    }

    loc, size := s.patFinder(data)
    if loc < 0 {
        return 0, nil, nil //request more data.
    }
    s.found = true
    debug("patFinder() loc=%d, size=%d, '%s'\n", loc, size, esc(data[loc:loc+size]))
    preLoc := 0
    preSize := 0
    if loc != 0 {
        var lastRsOffset int
        lastRsOffset, preSize = s.rsRevFinder(data[:loc])
        if lastRsOffset > 0 {
            preLoc = loc - lastRsOffset - preSize
        }
    }
    debug("rs='%s'\n", data[preLoc:preLoc+preSize])

    postLoc, postSize := s.rsFinder(data[loc+size:])
    if (postLoc < 0) {
        if (atEOF) {
            return len(data), data[preLoc:], nil
        } else {
            return 0, nil, nil //not enough data
        }
    }
    debug("postLoc, postSize = %d, %d\n", postLoc, postSize)
    debug("post string: %s\n", data[loc+size+postLoc:loc+size+postLoc+postSize])

    recBegin := preLoc+preSize
    recEnd   := loc+size+postLoc+postSize
    rec      := data[recBegin:recEnd]
    debug("RETURN: %d, %s\n", recEnd, esc(rec))
    return recEnd, rec, nil
}

//////////////////////////////////////////////////////////////////////////////
// Find-pattern-first algorithm

type SplitRecordFirstFinder struct {
    found bool;
    patFinder func(data []byte) (int, int)
    rsFinder func(data []byte) (int, int)
}

func NewSplitRecordFirstFinder(pat, rs string) *SplitRecordFirstFinder{
    //compile regex and set MLRFinder fields
    debug("rs=%s\n", rs)
    s := new(SplitRecordFirstFinder)
    s.patFinder = func(d []byte) (int, int) { return bytes.Index(d, []byte(pat)), len(pat) }
    s.rsFinder  = func(d []byte) (int, int) { return bytes.Index(d, []byte(rs)), len(rs) }
    return s
}

func (s *SplitRecordFirstFinder) Split(data []byte, atEOF bool) (advance int, token []byte, err error) {
    if atEOF && len(data) == 0 {
        return 0, nil, nil
    }
    pos, sz := s.rsFinder(data)
    if (pos < 0) {
        if (atEOF) {
            return len(data), data, nil
        } else {
            return 0, nil, nil //not enough data
        }
    }
    return pos+sz, data[0:pos+sz], nil
}



//////////////////////////////////////////////////////////////////////////////


//Split Record First
func grep_record(pat string, pipe chan string, wg* sync.WaitGroup) {
    defer wg.Done()
    for line := range pipe {
        if (strings.Index(line, pat) > 0) {
            fmt.Print(line)
        }
    }
}


func mlrgrep_srf(pat string, rs string, r io.Reader) {
    var wg sync.WaitGroup
    w := bufio.NewWriter(os.Stdout)
    pipe := make(chan string)
    scanner := bufio.NewScanner(r)
    splitter := NewSplitRecordFirstFinder(pat, rs)

    scanner.Split(splitter.Split)
    wg.Add(1)
    go grep_record(pat, pipe, &wg)

    for scanner.Scan() {
        line := scanner.Text()
        pipe <- line
    }
    close(pipe)
    wg.Wait()
    w.Flush()
}

//Find Pattern First
func mlrgrep_fpf(pat string, rs string, r io.Reader) {
    w := bufio.NewWriter(os.Stdout)
    scanner := NewScanner(r)
    splitter := NewPatternFirstFinder(pat, rs)

    scanner.Split(splitter.Split)

    for scanner.Scan() {
        line := scanner.Bytes()
        if splitter.found {
            w.Write(line)
        }
    }
    w.Flush()
}



func main() {
    goopt.Description = func() string {
        return "Example program for using the goopt flag library."
    }
    goopt.Version = "0.1"
    goopt.Usage = func() string {
        usage := "Usage: " + Usage
        usage += fmt.Sprintf("%s", Summary)
        usage += fmt.Sprintf("\n%s", goopt.Help())
        return usage
    }
    goopt.Parse(nil)

    var regex []string;
    var files []string;
    //defer fmt.Print("\033[0m") // defer resetting the terminal to default colors

    debug("os.Args: %s\n", os.Args)
    debug("rs=%s\n", *rs)

    i := 0;
    for _, a := range goopt.Args[i:] {
        if (a == "--") {
            i++
            break;
        }
        // if an argument is a filename for existing one,
        // assume that (and everything follows) as filename.
        _, err := os.Stat(a)
        if (err == nil) {
            break;
        }
        regex = append(regex, a)
        i++
    }

    for _, a := range goopt.Args[i:] {
        files = append(files, a)
    }
    debug("regex: %s\n", regex)
    debug("files: %s\n", files)

    //re := regexp.MustCompile(regex[0])
    for _, f := range files {
        //fgrep(regex[0], f)
        file, e := os.Open(f)
        checkError(e)
        defer file.Close()
        mlrgrep_srf(regex[0], *rs, file)
    }
}
