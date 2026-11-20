package fights

import (
	"errors"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"io"
	"testing"
)

func TestReadRejectsSwallowedSourceError(t *testing.T) {
	raw := &onceFailedReader{}
	decoder := func(r io.Reader, emit func(model.Event)) error {
		_, _ = io.ReadAll(r)
		for _, e := range boss(7, 1000) {
			emit(e)
		}
		return nil
	}
	result, err := Read(raw, decoder, params())
	if !errors.Is(err, io.ErrUnexpectedEOF) || len(result.Runs) > 0 || result.Source.SHA256 != "" {
		t.Fatal("swallowed source error published partial-file hash")
	}
}

type onceFailedReader struct{ failed bool }

func (r *onceFailedReader) Read(p []byte) (int, error) {
	if r.failed {
		return 0, io.EOF
	}
	r.failed = true
	return copy(p, "cut"), io.ErrUnexpectedEOF
}
