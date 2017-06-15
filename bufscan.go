// Took from Go 1.8 ditro's bufio/scan.go
// Original copyright below:

// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
    "errors"
    "io"
)


// Scanner provides a convenient interface for reading data such as
// a file of newline-delimited lines of text. Successive calls to
// the Scan method will step through the 'tokens' of a file, skipping
// the bytes between the tokens. The specification of a token is
// defined by a split function of type SplitFunc; the default split
// function breaks the input into lines with line termination stripped. Split
// functions are defined in this package for scanning a file into
// lines, bytes, UTF-8-encoded runes, and space-delimited words. The
// client may instead provide a custom split function.
//
// Scanning stops unrecoverably at EOF, the first I/O error, or a token too
// large to fit in the buffer. When a scan stops, the reader may have
// advanced arbitrarily far past the last token. Programs that need more
// control over error handling or large tokens, or must run sequential scans
// on a reader, should use bufio.Reader instead.
//
type Scanner struct {
    r            io.Reader // The reader provided by the client.
    split        SplitFunc // The function to split the tokens.
    maxTokenSize int       // Maximum size of a token; modified by tests.
    token        []byte    // Last token returned by split.
    buf          []byte    // Buffer used as argument to split.
    start        int       // First non-processed byte in buf.
    end          int       // End of data in buf.
    err          error     // Sticky error.
    empties      int       // Count of successive empty tokens.
    scanCalled   bool      // Scan has been called; buffer is in use.
}

// SplitFunc is the signature of the split function used to tokenize the
// input. The arguments are an initial substring of the remaining unprocessed
// data and a flag, atEOF, that reports whether the Reader has no more data
// to give. The return values are the number of bytes to advance the input
// and the next token to return to the user, plus an error, if any. If the
// data does not yet hold a complete token, for instance if it has no newline
// while scanning lines, SplitFunc can return (0, nil, nil) to signal the
// Scanner to read more data into the slice and try again with a longer slice
// starting at the same point in the input.
//
// If the returned error is non-nil, scanning stops and the error
// is returned to the client.
//
// The function is never called with an empty data slice unless atEOF
// is true. If atEOF is true, however, data may be non-empty and,
// as always, holds unprocessed text.
type SplitFunc func(data []byte, atEOF bool, tooLong bool) (advance int, token []byte, err error)

// Errors returned by Scanner.
var (
    ErrTooLong         = errors.New("bufio.Scanner: token too long")
    ErrNegativeAdvance = errors.New("bufio.Scanner: SplitFunc returns negative advance count")
    ErrAdvanceTooFar   = errors.New("bufio.Scanner: SplitFunc returns advance count beyond input")
)

const (
    // MaxScanTokenSize is the maximum size used to buffer a token
    // unless the user provides an explicit buffer with Scan.Buffer.
    // The actual maximum token size may be smaller as the buffer
    // may need to include, for instance, a newline.
    MaxScanTokenSize = 64 * 1024
    startBufSize = 4096 // Size of initial allocation for buffer.
    maxConsecutiveEmptyReads = 100
)

// NewScanner returns a new Scanner to read from r.
func NewScanner(r io.Reader) *Scanner {
    return &Scanner{
        r:            r,
        split:        nil,
        maxTokenSize: MaxScanTokenSize,
    }
}

// Err returns the first non-EOF error that was encountered by the Scanner.
func (s *Scanner) Err() error {
    if s.err == io.EOF {
        return nil
    }
    return s.err
}

// Bytes returns the most recent token generated by a call to Scan.
// The underlying array may point to data that will be overwritten
// by a subsequent call to Scan. It does no allocation.
func (s *Scanner) Bytes() []byte {
    return s.token
}

// Text returns the most recent token generated by a call to Scan
// as a newly allocated string holding its bytes.
func (s *Scanner) Text() string {
    return string(s.token)
}

// Scan advances the Scanner to the next token, which will then be
// available through the Bytes or Text method. It returns false when the
// scan stops, either by reaching the end of the input or an error.
// After Scan returns false, the Err method will return any error that
// occurred during scanning, except that if it was io.EOF, Err
// will return nil.
// Scan panics if the split function returns 100 empty tokens without
// advancing the input. This is a common error mode for scanners.
func (s *Scanner) Scan() bool {
    s.scanCalled = true
    // Loop until we have a token.
    tooLong := false;
    for {
        // See if we can get a token with what we already have.
        // If we've run out of data but have an error, give the split function
        // a chance to recover any remaining, possibly empty token.
        if s.end > s.start || s.err != nil {
            advance, token, err := s.split(s.buf[s.start:s.end],
                                           s.err != nil,
                                           tooLong)
            if err != nil {
                s.setErr(err)
                return false
            }
            if !s.advance(advance) {
                return false
            }
            s.token = token
            if token != nil {
                if s.err == nil || advance > 0 {
                    s.empties = 0
                } else {
                    // Returning tokens not advancing input at EOF.
                    s.empties++
                    if s.empties > 100 {
                        panic("bufio.Scan: 100 empty tokens without progressing")
                    }
                }
                return true
            }
        }
        // We cannot generate a token with what we are holding.
        // If we've already hit EOF or an I/O error, we are done.
        if s.err != nil {
            // Shut it down.
            s.start = 0
            s.end = 0
            return false
        }
        // Must read more data.
        // First, shift data to beginning of buffer if there's lots of empty space
        // or space is needed.
        if s.start > 0 && (s.end == len(s.buf) || s.start > len(s.buf)/2) {
            copy(s.buf, s.buf[s.start:s.end])
            s.end -= s.start
            s.start = 0
        }
        // Is the buffer full? If so, resize.
        if s.end == len(s.buf) {
            // Guarantee no overflow in the multiplication below.
            const maxInt = int(^uint(0) >> 1)
            if len(s.buf) >= s.maxTokenSize || len(s.buf) > maxInt/2 {
                tooLong = true
                continue //retry split() with tooLong flag set true
            }
            newSize := len(s.buf) * 2
            if newSize == 0 {
                newSize = startBufSize
            }
            if newSize > s.maxTokenSize {
                newSize = s.maxTokenSize
            }
            newBuf := make([]byte, newSize)
            copy(newBuf, s.buf[s.start:s.end])
            s.buf = newBuf
            s.end -= s.start
            s.start = 0
        }
        // Finally we can read some input. Make sure we don't get stuck with
        // a misbehaving Reader. Officially we don't need to do this, but let's
        // be extra careful: Scanner is for safe, simple jobs.
        for loop := 0; ; {
            n, err := s.r.Read(s.buf[s.end:len(s.buf)])
            s.end += n
            if err != nil {
                s.setErr(err)
                break
            }
            if n > 0 {
                s.empties = 0
                break
            }
            loop++
            if loop > maxConsecutiveEmptyReads {
                s.setErr(io.ErrNoProgress)
                break
            }
        }
    }
}

// advance consumes n bytes of the buffer. It reports whether the advance was legal.
func (s *Scanner) advance(n int) bool {
    if n < 0 {
        s.setErr(ErrNegativeAdvance)
        return false
    }
    if n > s.end-s.start {
        s.setErr(ErrAdvanceTooFar)
        return false
    }
    s.start += n
    return true
}

// setErr records the first error encountered.
func (s *Scanner) setErr(err error) {
    if s.err == nil || s.err == io.EOF {
        s.err = err
    }
}

// Buffer sets the initial buffer to use when scanning and the maximum
// size of buffer that may be allocated during scanning. The maximum
// token size is the larger of max and cap(buf). If max <= cap(buf),
// Scan will use this buffer only and do no allocation.
//
// By default, Scan uses an internal buffer and sets the
// maximum token size to MaxScanTokenSize.
//
// Buffer panics if it is called after scanning has started.
func (s *Scanner) Buffer(buf []byte, max int) {
    if s.scanCalled {
        panic("Buffer called after Scan")
    }
    s.buf = buf[0:cap(buf)]
    s.maxTokenSize = max
}

// Split sets the split function for the Scanner.
// The default split function is ScanLines.
//
// Split panics if it is called after scanning has started.
func (s *Scanner) Split(split SplitFunc) {
    if s.scanCalled {
        panic("Split called after Scan")
    }
    s.split = split
}
