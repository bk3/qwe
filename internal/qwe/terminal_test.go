package qwe

import (
	"os"
	"testing"
)

func TestCharacterDevicesAreNotTerminals(t *testing.T) {
	f, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if isTerminal(f.Fd()) {
		t.Fatal("/dev/null must not trigger an interactive prompt")
	}
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()
	if isTerminal(r.Fd()) {
		t.Fatal("pipe must not trigger an interactive prompt")
	}
}
