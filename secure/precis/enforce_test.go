// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package precis

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"

	"golang.org/x/text/internal/testtext"
	"golang.org/x/text/transform"
)

type testCase struct {
	input  string
	output string
	err    error
}

func doTests(t *testing.T, fn func(t *testing.T, p *Profile, tc testCase)) {
	for _, g := range enforceTestCases {
		for i, tc := range g.cases {
			name := fmt.Sprintf("%s:%d:%+q", g.name, i, tc.input)
			t.Run(name, func(t *testing.T) {
				fn(t, g.p, tc)
			})
		}
	}
}

func TestString(t *testing.T) {
	doTests(t, func(t *testing.T, p *Profile, tc testCase) {
		if e, err := p.String(tc.input); tc.err != err || e != tc.output {
			t.Errorf("got %+q (err: %v); want %+q (err: %v)", e, err, tc.output, tc.err)
		}
	})
}

func TestBytes(t *testing.T) {
	doTests(t, func(t *testing.T, p *Profile, tc testCase) {
		if e, err := p.Bytes([]byte(tc.input)); tc.err != err || string(e) != tc.output {
			t.Errorf("got %+q (err: %v); want %+q (err: %v)", string(e), err, tc.output, tc.err)
		}
	})

	t.Run("Copy", func(t *testing.T) {
		// Test that calling Bytes with something that doesn't transform returns a
		// copy.
		orig := []byte("hello")
		b, _ := NewFreeform().Bytes(orig)
		if reflect.ValueOf(b).Pointer() == reflect.ValueOf(orig).Pointer() {
			t.Error("original and result are the same slice; should be a copy")
		}
	})
}

func TestAppend(t *testing.T) {
	doTests(t, func(t *testing.T, p *Profile, tc testCase) {
		if e, err := p.Append(nil, []byte(tc.input)); tc.err != err || string(e) != tc.output {
			t.Errorf("got %+q (err: %v); want %+q (err: %v)", string(e), err, tc.output, tc.err)
		}
	})
}

func TestStringMallocs(t *testing.T) {
	if n := testtext.AllocsPerRun(100, func() { UsernameCaseMapped.String("helloworld") }); n > 0 {
		// TODO: reduce this to 0.
		t.Skipf("got %f allocs, want 0", n)
	}
}

func TestAppendMallocs(t *testing.T) {
	str := []byte("helloworld")
	out := make([]byte, 0, len(str))
	if n := testtext.AllocsPerRun(100, func() { UsernameCaseMapped.Append(out, str) }); n > 0 {
		t.Errorf("got %f allocs, want 0", n)
	}
}

func TestTransformMallocs(t *testing.T) {
	str := []byte("helloworld")
	out := make([]byte, 0, len(str))
	tr := UsernameCaseMapped.NewTransformer()
	if n := testtext.AllocsPerRun(100, func() {
		tr.Reset()
		tr.Transform(out, str, true)
	}); n > 0 {
		t.Errorf("got %f allocs, want 0", n)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// TestTransformerShortBuffers tests that the precis.Transformer implements the
// spirit, not just the letter (the method signatures), of the
// transform.Transformer interface.
//
// In particular, it tests that, if one or both of the dst or src buffers are
// short, so that multiple Transform calls are required to complete the overall
// transformation, the end result is identical to one Transform call with
// sufficiently long buffers.
func TestTransformerShortBuffers(t *testing.T) {
	testCases := []struct {
		name       string
		srcUnit    []byte
		wantUnit   []byte
		srcRepeat  int
		wantRepeat int
		srcLen     int
		wantLen    int
		wantErr    error
		tr         *Transformer
	}{
		{
			name:       "Freeform",
			srcUnit:    []byte("a\u0300cce\u0301nts"), // NFD normalization form
			wantUnit:   []byte("àccénts"),             // NFC normalization form
			srcRepeat:  16,
			wantRepeat: 16,
			srcLen:     11,
			wantLen:    9,
			tr:         NewFreeform().NewTransformer(),
		},
		{
			name:       "Nickname1",
			srcUnit:    []byte("a\u0300cce\u0301nts"),
			wantUnit:   []byte("àccénts"),
			srcRepeat:  16,
			wantRepeat: 16,
			srcLen:     11,
			wantLen:    9,
			tr:         Nickname.NewTransformer(),
		},
		{
			name:       "Nickname2",
			srcUnit:    []byte("ﷺ"), // U+FDFA
			wantUnit:   []byte("صلى الله عليه وسلم"),
			srcRepeat:  16,
			wantRepeat: 16,
			srcLen:     3,
			wantLen:    33,
			tr:         Nickname.NewTransformer(),
		},
		{
			name:       "incomplete 3-byte rune",
			srcUnit:    []byte("a\xef\xb7"), // input ends in an incomplete 3-byte rune
			wantUnit:   []byte("a"),
			srcRepeat:  1,
			wantRepeat: 1,
			srcLen:     3,
			wantLen:    1,
			wantErr:    errDisallowedRune,
			tr:         Nickname.NewTransformer(),
		},
		{
			name:       "bad utf8",
			srcUnit:    []byte("a\xff"), // \xff is never valid
			wantUnit:   []byte("a"),
			srcRepeat:  1,
			wantRepeat: 1,
			srcLen:     2,
			wantLen:    1,
			wantErr:    errDisallowedRune,
			tr:         Nickname.NewTransformer(),
		},
		{
			name:       "Nickname space",
			srcUnit:    []byte("abcd b"),
			wantUnit:   []byte("abcd b"),
			srcRepeat:  16,
			wantRepeat: 16,
			srcLen:     6,
			wantLen:    6,
			tr:         Nickname.NewTransformer(),
		},
		{
			name:       "Nickname return nil",
			srcUnit:    []byte("a b"),
			wantUnit:   []byte("a b"),
			srcRepeat:  10,
			wantRepeat: 10,
			srcLen:     3,
			wantLen:    3,
			tr:         Nickname.NewTransformer(),
		},
		{
			name:       "Nickname partial",
			srcUnit:    []byte("ﷺ"), // U+FDFA (3 bytes)
			wantUnit:   []byte("ﷺ"),
			srcRepeat:  16,
			wantRepeat: 16,
			srcLen:     3,
			wantLen:    3,
			tr:         &Transformer{&nickAdditionalMapping{}},
		},
		{
			name:       "context boundary",
			srcUnit:    []byte("\u0628\u200c\u0627"), // JoiningD + ZWNJ + JoiningR (2+3+2=7 bytes)
			wantUnit:   []byte("\u0628\u200c\u0627"),
			srcRepeat:  16,
			wantRepeat: 16,
			srcLen:     7,
			wantLen:    7,
			tr:         NewFreeform().NewTransformer(),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			src := bytes.Repeat(tc.srcUnit, tc.srcRepeat)
			want := bytes.Repeat(tc.wantUnit, tc.wantRepeat)
			const long = 4096
			dst := make([]byte, long)

			// 5, 7, 9, 11, 13, 16 and 17 are all pair-wise co-prime, which means that
			// slicing the dst and src buffers into 5, 7, 13 and 17 byte chunks will
			// fall at different places inside the repeated srcUnit's and wantUnit's.
			if len(tc.srcUnit) != tc.srcLen || len(tc.wantUnit) != tc.wantLen || len(src) > long || len(want) > long {
				t.Fatal("inconsistent lengths")
			}
			for _, deltaD := range []int{5, 7, 13, 17, long} {
			loop:
				for _, deltaS := range []int{5, 7, 13, 17, long} {
					tc.tr.Reset()
					d0 := 0
					s0 := 0
					var nDst, nSrc int
					var err error
					for {
						d1 := min(len(dst), d0+deltaD)
						s1 := min(len(src), s0+deltaS)
						nDst, nSrc, err = tc.tr.Transform(dst[d0:d1:d1], src[s0:s1:s1], s1 == len(src))
						d0 += nDst
						s0 += nSrc
						if err == nil && s0 == len(src) {
							break
						}
						if err == nil {
							continue
						}
						if err == transform.ErrShortDst || (err == transform.ErrShortSrc && s1 < len(src)) {
							continue
						}
						if err == tc.wantErr && (s1 == len(src) || tc.wantErr != transform.ErrShortSrc) {
							break
						}
						t.Errorf("deltaD=%d, deltaS=%d: %v", deltaD, deltaS, err)
						continue loop
					}
					if tc.wantErr == nil && s0 != len(src) {
						t.Errorf("deltaD=%d, deltaS=%d: s0: got %d, want %d", deltaD, deltaS, s0, len(src))
						continue
					}
					if d0 != len(want) {
						t.Errorf("deltaD=%d, deltaS=%d: d0: got %d, want %d", deltaD, deltaS, d0, len(want))
						continue
					}
					got := dst[:d0]
					if !bytes.Equal(got, want) {
						t.Errorf("deltaD=%d, deltaS=%d:\ngot  %q\nwant %q", deltaD, deltaS, got, want)
						continue
					}
					if err != tc.wantErr {
						t.Errorf("incorrect error: got %t, want %t", err, tc.wantErr)
					}
				}
			}
		})
	}
}

// TestNicknameInternalBufferBoundary tests that the Nickname profile's
// Transformer does not panic when its additional mapping step runs out of room
// in the internal buffers of the transform chain in the middle of a multi-byte
// rune, or right before the interior space it has to emit.
func TestNicknameInternalBufferBoundary(t *testing.T) {
	const bufSize = 4096 // size of the internal buffers of transform.Chain.
	testCases := []struct {
		name string
		src  []byte
	}{
		// The 2-byte rune é straddles the end of the internal buffer.
		{"2-byte rune", append(bytes.Repeat([]byte("a"), bufSize-1), "é"...)},
		// The 3-byte rune 世 straddles the end of the internal buffer.
		{"3-byte rune", append(bytes.Repeat([]byte("a"), bufSize-2), "世"...)},
		// The interior space has to be written just past the end of the
		// internal buffer.
		{"interior space", append(bytes.Repeat([]byte("a"), bufSize), " b"...)},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("Transform panicked: %v", r)
				}
			}()
			want := tc.src
			dst := make([]byte, 2*len(tc.src))
			tr := Nickname.NewTransformer()
			nDst, nSrc, err := tr.Transform(dst, tc.src, true)
			if err != nil {
				t.Fatalf("Transform: unexpected error: %v", err)
			}
			if nSrc != len(tc.src) {
				t.Errorf("nSrc: got %d, want %d", nSrc, len(tc.src))
			}
			if got := dst[:nDst]; !bytes.Equal(got, want) {
				t.Errorf("got %d bytes %q...%q, want %d bytes %q...%q",
					len(got), got[:min(len(got), 8)], got[len(got)-min(len(got), 8):],
					len(want), want[:8], want[len(want)-8:])
			}
		})
	}
}
