// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package norm

import (
	"strings"
	"testing"
)

func doIterNormString(f Form, s string) []byte {
	acc := []byte{}
	i := Iter{}
	i.InitString(f, s)
	for !i.Done() {
		acc = append(acc, i.Next()...)
	}
	return acc
}

func doIterNorm(f Form, s string) []byte {
	acc := []byte{}
	i := Iter{}
	i.Init(f, []byte(s))
	for !i.Done() {
		acc = append(acc, i.Next()...)
	}
	return acc
}

func TestIterNext(t *testing.T) {
	runNormTests(t, "IterNext", func(f Form, out []byte, s string) []byte {
		return doIterNormString(f, string(append(out, s...)))
	})
	runNormTests(t, "IterNext", func(f Form, out []byte, s string) []byte {
		return doIterNorm(f, string(append(out, s...)))
	})
}

type SegmentTest struct {
	in  string
	out []string
}

var segmentTests = []SegmentTest{
	{"\u1E0A\u0323a", []string{"\x44\u0323\u0307", "a", ""}},
	{rep('a', segSize), append(strings.Split(rep('a', segSize), ""), "")},
	{rep('a', segSize+2), append(strings.Split(rep('a', segSize+2), ""), "")},
	{rep('a', segSize) + "\u0300aa",
		append(strings.Split(rep('a', segSize-1), ""), "a\u0300", "a", "a", "")},

	// U+0f73 is NOT treated as a starter as it is a modifier
	{"a" + grave(29) + "\u0f73", []string{"a" + grave(29), cgj + "\u0f73"}},
	{"a\u0f73", []string{"a\u0f73"}},

	// U+ff9e is treated as a non-starter.
	// TODO: should we? Note that this will only affect iteration, as whether
	// or not we do so does not affect the normalization output and will either
	// way result in consistent iteration output.
	{"a" + grave(30) + "\uff9e", []string{"a" + grave(30), cgj + "\uff9e"}},
	{"a\uff9e", []string{"a\uff9e"}},
}

var segmentTestsK = []SegmentTest{
	{"\u3332", []string{"\u30D5", "\u30A1", "\u30E9", "\u30C3", "\u30C8\u3099", ""}},
	// last segment of multi-segment decomposition needs normalization
	{"\u3332\u093C", []string{"\u30D5", "\u30A1", "\u30E9", "\u30C3", "\u30C8\u093C\u3099", ""}},
	{"\u320E", []string{"\x28", "\uAC00", "\x29"}},

	// last segment should be copied to start of buffer.
	{"\ufdfa", []string{"\u0635", "\u0644", "\u0649", " ", "\u0627", "\u0644", "\u0644", "\u0647", " ", "\u0639", "\u0644", "\u064a", "\u0647", " ", "\u0648", "\u0633", "\u0644", "\u0645", ""}},
	{"\ufdfa" + grave(30), []string{"\u0635", "\u0644", "\u0649", " ", "\u0627", "\u0644", "\u0644", "\u0647", " ", "\u0639", "\u0644", "\u064a", "\u0647", " ", "\u0648", "\u0633", "\u0644", "\u0645" + grave(30), ""}},
	{"\uFDFA" + grave(64), []string{"\u0635", "\u0644", "\u0649", " ", "\u0627", "\u0644", "\u0644", "\u0647", " ", "\u0639", "\u0644", "\u064a", "\u0647", " ", "\u0648", "\u0633", "\u0644", "\u0645" + grave(30), cgj + grave(30), cgj + grave(4), ""}},

	// Hangul and Jamo are grouped together.
	{"\uAC00", []string{"\u1100\u1161", ""}},
	{"\uAC01", []string{"\u1100\u1161\u11A8", ""}},
	{"\u1100\u1161", []string{"\u1100\u1161", ""}},
}

// Note that, by design, segmentation is equal for composing and decomposing forms.
func TestIterSegmentation(t *testing.T) {
	segmentTest(t, "SegmentTestD", NFD, segmentTests)
	segmentTest(t, "SegmentTestC", NFC, segmentTests)
	segmentTest(t, "SegmentTestKD", NFKD, segmentTestsK)
	segmentTest(t, "SegmentTestKC", NFKC, segmentTestsK)
}

// iterInvalidUTF8Tests holds inputs that contain invalid UTF-8 together with
// the expected iteration result for NFC, NFD, NFKC and NFKD.
//
// Invalid runes used to be given a Properties with a size of 0. As nextComposed
// advances its input by the size of the rune it just processed, a zero size
// meant Iter.Next could return an empty segment without advancing i.p: Done
// never started reporting true and a caller looping until Done spun forever.
// compInfo now hands out a size of 1 for invalid runes, so iteration always
// makes progress. See golang/go#80142.
var iterInvalidUTF8Tests = []struct {
	in  string
	out [4]string // expected output for NFC, NFD, NFKC and NFKD
}{
	// A truncated 4-byte sequence followed by a combining mark is the shortest
	// input that used to hang NFC and NFKC iteration.
	{
		"a\xf3\u0300",
		[4]string{"a\xf3\u0300", "a\xf3\u0300", "a\xf3\u0300", "a\xf3\u0300"},
	},
	{
		"a\xf0\u0300",
		[4]string{"a\xf0\u0300", "a\xf0\u0300", "a\xf0\u0300", "a\xf0\u0300"},
	},
	{
		"\xf3\u0300",
		[4]string{"\xf3\u0300", "\xf3\u0300", "\xf3\u0300", "\xf3\u0300"},
	},
	{
		"aa\xf3\u0300",
		[4]string{"aa\xf3\u0300", "aa\xf3\u0300", "aa\xf3\u0300", "aa\xf3\u0300"},
	},
	{
		"a\xf3\u0300\u0300",
		[4]string{
			"a\xf3\u0300\u0300",
			"a\xf3\u0300\u0300",
			"a\xf3\u0300\u0300",
			"a\xf3\u0300\u0300",
		},
	},
	// Truncated sequences that are not followed by anything.
	{
		"\xf3",
		[4]string{"\xf3", "\xf3", "\xf3", "\xf3"},
	},
	{
		"a\xf3",
		[4]string{"a\xf3", "a\xf3", "a\xf3", "a\xf3"},
	},
	// Illegal continuation bytes already had a non-zero size; verify that they
	// keep normalizing as before.
	{
		"a\xe1\u0300",
		[4]string{"a\xe1\u0300", "a\xe1\u0300", "a\xe1\u0300", "a\xe1\u0300"},
	},
	{
		"a\xc2\u0300",
		[4]string{"a\xc2\u0300", "a\xc2\u0300", "a\xc2\u0300", "a\xc2\u0300"},
	},
	// A truncated sequence preceded by a composable sequence.
	{
		"a\u0300\xf3",
		[4]string{"\u00e0\xf3", "a\u0300\xf3", "\u00e0\xf3", "a\u0300\xf3"},
	},
	// An invalid rune following a rune that has a decomposition.
	{
		"\u1e0a\xf3\u0300",
		[4]string{
			"\u1e0a\xf3\u0300",
			"D\u0307\xf3\u0300",
			"\u1e0a\xf3\u0300",
			"D\u0307\xf3\u0300",
		},
	},
	// An invalid rune following a multi-segment decomposition.
	{
		"\u3332\xf3\u0300",
		[4]string{
			"\u3332\xf3\u0300",
			"\u3332\xf3\u0300",
			"\u30d5\u30a1\u30e9\u30c3\u30c9\xf3\u0300",
			"\u30d5\u30a1\u30e9\u30c3\u30c8\u3099\xf3\u0300",
		},
	},
	// An invalid rune reached with a full non-starter buffer.
	{
		"a" + grave(maxNonStarters) + "\xf3\u0300",
		[4]string{
			"\u00e0" + grave(maxNonStarters-1) + "\xf3\u0300",
			"a" + grave(maxNonStarters) + "\xf3\u0300",
			"\u00e0" + grave(maxNonStarters-1) + "\xf3\u0300",
			"a" + grave(maxNonStarters) + "\xf3\u0300",
		},
	},
}

// iterCollect iterates over s and returns the concatenated segments. It gives
// up after more iterations than an input of this size can legitimately need, so
// that an iterator that fails to advance is reported as a failure instead of
// hanging the test binary.
func iterCollect(f Form, s string, useBytes bool) (acc []byte, done bool) {
	iter := Iter{}
	if useBytes {
		iter.Init(f, []byte(s))
	} else {
		iter.InitString(f, s)
	}
	acc = []byte{}
	for n := 0; !iter.Done(); n++ {
		if n > 8*len(s)+64 {
			return acc, false
		}
		acc = append(acc, iter.Next()...)
	}
	return acc, true
}

func TestIterInvalidUTF8(t *testing.T) {
	forms := []Form{NFC, NFD, NFKC, NFKD}
	names := []string{"NFC", "NFD", "NFKC", "NFKD"}
	for i, tt := range iterInvalidUTF8Tests {
		for j, f := range forms {
			for _, useBytes := range []bool{false, true} {
				res, done := iterCollect(f, tt.in, useBytes)
				if !done {
					t.Errorf("%s:%d:bytes=%v: Iter did not terminate on %+q; got %+q so far",
						names[j], i, useBytes, tt.in, pc(string(res)))
					continue
				}
				if got := string(res); got != tt.out[j] {
					t.Errorf("%s:%d:bytes=%v: was %+q; want %+q",
						names[j], i, useBytes, pc(got), pc(tt.out[j]))
				}
			}
		}
	}
}

// TestIterInvalidUTF8Progress exhaustively verifies that iteration terminates
// for every short combination of bytes that is prone to producing invalid
// runes. Over a thousand of these needed an unbounded number of Iter.Next calls
// before golang/go#80142 was fixed.
func TestIterInvalidUTF8Progress(t *testing.T) {
	// ASCII, continuation bytes and 2-, 3- and 4-byte leaders. A leader that
	// runs past the end of the input yields a truncated, and hence invalid,
	// rune.
	alphabet := []byte{'a', 0x80, 0xbf, 0xc2, 0xcc, 0xc3, 0xe0, 0xe1, 0xf0, 0xf3, 0xff}
	forms := []Form{NFC, NFD, NFKC, NFKD}
	names := []string{"NFC", "NFD", "NFKC", "NFKD"}
	failed := 0
	var walk func(b []byte, depth int)
	walk = func(b []byte, depth int) {
		if len(b) > 0 {
			for j, f := range forms {
				for _, useBytes := range []bool{false, true} {
					if _, done := iterCollect(f, string(b), useBytes); !done {
						if failed++; failed <= 10 {
							t.Errorf("%s:bytes=%v: Iter did not terminate on %+q",
								names[j], useBytes, string(b))
						}
					}
				}
			}
		}
		if depth == 0 {
			return
		}
		for _, c := range alphabet {
			walk(append(b, c), depth-1)
		}
	}
	walk(nil, 4)
	if failed > 10 {
		t.Errorf("Iter did not terminate for %d inputs in total", failed)
	}
}

func segmentTest(t *testing.T, name string, f Form, tests []SegmentTest) {
	iter := Iter{}
	for i, tt := range tests {
		iter.InitString(f, tt.in)
		for j, seg := range tt.out {
			if seg == "" {
				if !iter.Done() {
					res := string(iter.Next())
					t.Errorf(`%s:%d:%d: expected Done()==true, found segment %+q`, name, i, j, res)
				}
				continue
			}
			if iter.Done() {
				t.Errorf("%s:%d:%d: Done()==true, want false", name, i, j)
			}
			seg = f.String(seg)
			if res := string(iter.Next()); res != seg {
				t.Errorf(`%s:%d:%d" segment was %+q (%d); want %+q (%d)`, name, i, j, pc(res), len(res), pc(seg), len(seg))
			}
		}
	}
}
