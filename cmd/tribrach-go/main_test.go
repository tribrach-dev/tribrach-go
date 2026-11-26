package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const src = `package p

//tribrach:link M.step
func Step() {}

//tribrach:link M.other
//tribrach:stamp 1111111111111111
func Other() {}

//tribrach:link M.same
//tribrach:stamp 3333333333333333
func Same() {}
`

const manifestJSON = `{"format": "tribrach.manifest.v1", "package": "p", "links": [
 {"lean": "M.step", "owner": "tribrach.go", "symbol": "example.com/p.Step", "stamp": "0123456789abcdef"},
 {"lean": "M.other", "owner": "tribrach.go", "symbol": "example.com/p.Other", "stamp": "2222222222222222"},
 {"lean": "M.same", "owner": "tribrach.go", "symbol": "example.com/p.Same", "stamp": "3333333333333333"}]}`

func TestStamp(t *testing.T) {
	dir := t.TempDir()
	write := func(name, text string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module example.com/p\n\ngo 1.24.0\n")
	write("p.go", src)
	write("tribrach.manifest.json", manifestJSON)
	wd, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(wd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	code, err := stamp("", true, []string{"./..."})
	if err != nil || code != 1 {
		t.Fatalf("dry run: code %d, err %v; want 1 (two stale)", code, err)
	}
	if code, err := stamp("", false, []string{"./..."}); err != nil || code != 0 {
		t.Fatalf("stamp: code %d, err %v", code, err)
	}
	got, _ := os.ReadFile("p.go")
	want := strings.NewReplacer(
		"//tribrach:link M.step\n", "//tribrach:link M.step\n//tribrach:stamp 0123456789abcdef\n",
		"1111111111111111", "2222222222222222",
	).Replace(src)
	if string(got) != want {
		t.Errorf("after stamping:\n%s\nwant:\n%s", got, want)
	}
	if code, err := stamp("", true, []string{"./..."}); err != nil || code != 0 {
		t.Fatalf("second dry run: code %d, err %v; want 0", code, err)
	}
}
