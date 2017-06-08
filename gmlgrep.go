package main

import (
    "fmt"
    "os"
    "bufio"
    "regexp"
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



func fgrep(pattern string, fileName string) bool {
    found := false

    file, e := os.Open(fileName)
    checkError(e)
    defer file.Close()

    scanner := bufio.NewScanner(file)
    for scanner.Scan() {
        line := scanner.Text()
        // FIXME: a patch for standard string package is needed to export StringFInd()
        if strings.StringFind(pattern, line) >= 0 {
        //if strings.Index(line, pattern) >= 0 {
            fmt.Print(scanner.Text())
            fmt.Print("\n")
        }
    }
    return found
}


func grep(re *regexp.Regexp, fileName string) bool {
    found := false

    file, e := os.Open(fileName)
    checkError(e)
    defer file.Close()

    scanner := bufio.NewScanner(file)
    for scanner.Scan() {
        line := scanner.Text()
        if re.MatchString(line) {
            fmt.Print(scanner.Text())
            fmt.Print("\n")
        }
    }
    return found
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
    defer fmt.Print("\033[0m") // defer resetting the terminal to default colors

    fmt.Printf("os.Args: %s\n", os.Args)

    i := 1;
    for _, a := range os.Args[1:] {
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

    for _, a := range os.Args[i:] {
        files = append(files, a)
    }
    fmt.Printf("regex: %s\n", regex)
    fmt.Printf("files: %s\n", files)

    //re := regexp.MustCompile(regex[0])
    for _, f := range files {
        fgrep(regex[0], f)
    }

}

func checkError(e error) {
    if e != nil {
        fmt.Println(e)
        os.Exit(1)
    }
}
