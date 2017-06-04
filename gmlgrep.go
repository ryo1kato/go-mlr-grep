package main

import (
    "fmt"
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
    defer fmt.Print("\033[0m") // defer resetting the terminal to default colors
    /*
    switch *color {
    case "default":
    case "red": fmt.Print("\033[31m")
    case "green": fmt.Print("\033[32m")
    case "blue": fmt.Print("\033[34m")
    default: panic("Unrecognized color!") // this should never happen!
    }
    for i:=0; i<*repetitions; i++ {
        fmt.Println("Greetings,", *username)
        log("You have", *repetitions, "children.")
        for _,child := range *children {
            fmt.Println("I also greet your child, whose name is", child)
        }
    }
    */
}
