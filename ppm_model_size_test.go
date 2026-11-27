package rardecode

import (
	"errors"
	"io"
	"os"
	"runtime"
	"testing"
)

var ppmFixtures = []string{"testdata/testfile.rar3.rar", "testdata/testfile.rar3.solid.rar"}

const (
	fixtureDeclared = 25 << 20 // the PPMd block states a 25 MB model
	fixtureAlloc    = 34952560 // measured: states allocated for it, 8 bytes each
	fixtureContent  = "Testing 123\n"
)

// readAll reads every member; it returns the contents and the first error.
func readAll(path string, opts ...Option) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	r, err := NewReader(f, opts...)
	if err != nil {
		return "", err
	}
	var out string
	for {
		if _, err := r.Next(); err != nil {
			if err == io.EOF {
				err = nil
			}
			return out, err
		}
		b, err := io.ReadAll(r)
		out += string(b)
		if err != nil {
			return out, err
		}
	}
}

func TestPPMModelSizeSeesDeclaredSize(t *testing.T) {
	for _, p := range ppmFixtures {
		var calls [][2]int64
		got, err := readAll(p, PPMModelSize(func(declared, alloc int64) error {
			calls = append(calls, [2]int64{declared, alloc})
			return nil
		}))
		if err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		if got != fixtureContent {
			t.Errorf("%s: content %q", p, got)
		}
		if len(calls) != 1 || calls[0] != [2]int64{fixtureDeclared, fixtureAlloc} {
			t.Errorf("%s: calls %v, want one call (%d, %d)", p, calls, fixtureDeclared, fixtureAlloc)
		}
	}
}

func TestPPMModelSizeNoOptionIsUnchanged(t *testing.T) {
	for _, p := range ppmFixtures {
		plain, err := readAll(p)
		if err != nil || plain != fixtureContent {
			t.Fatalf("%s: no option: %q, %v", p, plain, err)
		}
		hooked, err := readAll(p, PPMModelSize(func(int64, int64) error { return nil }))
		if err != nil || hooked != plain {
			t.Errorf("%s: with a function that accepts: %q, %v; no option: %q", p, hooked, err, plain)
		}
	}
}

func TestPPMModelSizeRefusalAllocatesNoModel(t *testing.T) {
	refuse := errors.New("does not fit")
	for _, p := range ppmFixtures {
		f, err := os.Open(p)
		if err != nil {
			t.Fatal(err)
		}
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		r, err := NewReader(f, PPMModelSize(func(int64, int64) error { return refuse }))
		if err != nil {
			t.Fatal(err)
		}
		_, err = r.Next()
		var first error
		if err == nil {
			_, first = io.ReadAll(r)
		} else {
			first = err
		}
		runtime.ReadMemStats(&after)
		f.Close()
		var se *PPMModelSizeError
		if !errors.As(first, &se) || !errors.Is(first, refuse) {
			t.Fatalf("%s: error %v, want *PPMModelSizeError wrapping the refusal", p, first)
		}
		if se.Declared != fixtureDeclared || se.Alloc != fixtureAlloc {
			t.Errorf("%s: error holds %d, %d", p, se.Declared, se.Alloc)
		}
		// TotalAlloc only grows; the model would add fixtureAlloc (34.9 MB) to it.
		if d := after.TotalAlloc - before.TotalAlloc; d > fixtureAlloc/2 {
			t.Errorf("%s: %d bytes allocated across a refused read; the model alone is %d", p, d, fixtureAlloc)
		}
		// a later Read returns the refusal again, and does not panic or allocate the model
		buf := make([]byte, 16)
		for i := 0; i < 2; i++ {
			if _, err := r.Read(buf); !errors.Is(err, refuse) {
				t.Errorf("%s: later Read %d: %v, want the refusal again", p, i, err)
			}
		}
		var again runtime.MemStats
		runtime.ReadMemStats(&again)
		if d := again.TotalAlloc - after.TotalAlloc; d > fixtureAlloc/2 {
			t.Errorf("%s: %d bytes allocated by later Reads", p, d)
		}
	}
}
