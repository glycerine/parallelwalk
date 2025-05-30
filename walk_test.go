// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pwalk_test

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"

	walk "github.com/glycerine/parallelwalk"
)

var _ = fmt.Printf

var LstatP = walk.LstatP

type Node struct {
	name    string
	entries []*Node // nil if the entry is a file
	mark    int
}

var tree = &Node{
	"testdata",
	[]*Node{
		{"a", nil, 0},
		{"b", []*Node{}, 0},
		{"c", nil, 0},
		{
			"d",
			[]*Node{
				{"x", nil, 0},
				{"y", []*Node{}, 0},
				{
					"z",
					[]*Node{
						{"u", nil, 0},
						{"v", nil, 0},
					},
					0,
				},
			},
			0,
		},
	},
	0,
}

func walkTree(n *Node, path string, f func(path string, n *Node)) {
	f(path, n)
	for _, e := range n.entries {
		walkTree(e, walk.Join(path, e.name), f)
	}
}

func makeTree(t *testing.T) {
	walkTree(tree, tree.name, func(path string, n *Node) {
		if n.entries == nil {
			fd, err := os.Create(path)
			if err != nil {
				t.Errorf("makeTree: %v", err)
				return
			}
			fd.Close()
		} else {
			os.Mkdir(path, 0770)
		}
	})
}

func markTree(n *Node) { walkTree(n, "", func(path string, n *Node) { n.mark++ }) }

func checkMarks(t *testing.T, report bool) {
	walkTree(tree, tree.name, func(path string, n *Node) {
		if n.mark != 1 && report {
			t.Errorf("node %s mark = %d; expected 1", path, n.mark)
		}
		n.mark = 0
	})
}

var mut sync.Mutex

// Assumes that each node name is unique. Good enough for a test.
// If clear is true, any incoming error is cleared before return. The errors
// are always accumulated, though.
func mark(path string, info os.FileInfo, err error, errors *[]error, clear bool) error {
	if err != nil {
		// mark(path='testdata/d') given err = 'open testdata/d: permission denied'; clear = 'true'
		//fmt.Printf("mark(path='%v') given err = '%v'; clear = '%v'\n", path, err, clear)
		mut.Lock()
		*errors = append(*errors, err) // has data race with itself here. add mut
		mut.Unlock()
		if clear {
			return nil
		}
		return err
	}
	name := info.Name()
	walkTree(tree, tree.name, func(path string, n *Node) {
		if n.name == name {
			n.mark++
			//fmt.Printf("mark(path='%v') has marked name = '%v'\n", path, name)
		} else {
			//fmt.Printf("mark(path='%v') ignores '%v' = n.name != name = '%v'\n", path, n.name, name)
		}
	})
	return nil
}

func TestWalk(t *testing.T) {
	// try to clean up any prior runs, thought this
	// may not be enough if permissions borked(!)
	os.RemoveAll(tree.name)

	makeTree(t)
	errors := make([]error, 0, 10)
	clear := true
	markFn := func(path string, info os.FileInfo, subdir bool, err error) error {
		return mark(path, info, err, &errors, clear)
	}
	// Expect no errors.
	err := walk.Walk(tree.name, markFn)
	if err != nil {
		t.Fatalf("no error expected, found: %s", err)
	}
	if len(errors) != 0 {
		t.Fatalf("unexpected errors: %s", errors)
	}
	checkMarks(t, true)
	errors = errors[0:0]

	// Test permission errors. Handle WASM differently since it has a different permission model
	if runtime.GOOS == "wasip1" || runtime.GOOS == "js" {
		// For WASM, we'll simulate permission errors by making the directories inaccessible
		// through a different mechanism - by removing them
		dir1 := walk.Join(tree.name, tree.entries[1].name)
		dir2 := walk.Join(tree.name, tree.entries[3].name)

		// Save the contents so we can restore them
		os.Rename(dir1, dir1+".bak")
		os.Rename(dir2, dir2+".bak")

		// Mark respective subtrees manually since we can't access them
		markTree(tree.entries[1])
		markTree(tree.entries[3])

		err := walk.Walk(tree.name, markFn)
		if err != nil {
			t.Fatalf("expected no error return from Walk, got %s", err)
		}
		if len(errors) != 2 {
			t.Errorf("expected 2 errors, got %d: %s", len(errors), errors)
		}
		checkMarks(t, true)
		errors = errors[0:0]

		// Test stopping after first error
		markTree(tree.entries[1])
		markTree(tree.entries[3])
		tree.entries[1].mark--
		tree.entries[3].mark--
		clear = false // error will stop processing
		err = walk.Walk(tree.name, markFn)
		if err == nil {
			t.Fatalf("expected error return from Walk")
		}
		checkMarks(t, false)
		errors = errors[0:0]

		// Restore the directories
		os.Rename(dir1+".bak", dir1)
		os.Rename(dir2+".bak", dir2)
	} else if os.Getuid() > 0 && !testing.Short() {
		// Original permission test for non-WASM environments
		os.Chmod(walk.Join(tree.name, tree.entries[1].name), 0)
		os.Chmod(walk.Join(tree.name, tree.entries[3].name), 0)

		markTree(tree.entries[1])
		markTree(tree.entries[3])

		err := walk.Walk(tree.name, markFn)
		if err != nil {
			t.Fatalf("expected no error return from Walk, got %s", err)
		}
		if len(errors) != 2 {
			t.Errorf("expected 2 errors, got %d: %s", len(errors), errors)
		}
		checkMarks(t, true)
		errors = errors[0:0]

		markTree(tree.entries[1])
		markTree(tree.entries[3])
		tree.entries[1].mark--
		tree.entries[3].mark--
		clear = false // error will stop processing
		err = walk.Walk(tree.name, markFn)
		if err == nil {
			t.Fatalf("expected error return from Walk")
		}
		checkMarks(t, false)
		errors = errors[0:0]

		// restore permissions
		os.Chmod(walk.Join(tree.name, tree.entries[1].name), 0770)
		os.Chmod(walk.Join(tree.name, tree.entries[3].name), 0770)
	}

	// cleanup
	if err := os.RemoveAll(tree.name); err != nil {
		t.Errorf("removeTree: %v", err)
	}
}

func touch(t *testing.T, name string) {
	f, err := os.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestWalkFileError(t *testing.T) {
	// For WASM, we need to handle temp dir creation differently
	var td string
	var err error
	if runtime.GOOS == "wasip1" || runtime.GOOS == "js" {
		// In WASM, try to create a temp dir in the current directory
		td = "walktest_temp"
		err = os.MkdirAll(td, 0755)
		if err != nil {
			t.Skipf("Skipping TestWalkFileError under WASM - cannot create test directory: %v", err)
		}
	} else {
		// Normal OS behavior
		td, err = ioutil.TempDir("", "walktest")
		if err != nil {
			t.Fatal(err)
		}
	}
	defer os.RemoveAll(td)

	var mapmut sync.Mutex
	touch(t, walk.Join(td, "foo"))
	touch(t, walk.Join(td, "bar"))
	dir := walk.Join(td, "dir")
	if err := os.MkdirAll(walk.Join(td, "dir"), 0755); err != nil {
		t.Fatal(err)
	}
	touch(t, walk.Join(dir, "baz"))
	touch(t, walk.Join(dir, "stat-error"))
	defer func() {
		*walk.LstatP = os.Lstat
	}()
	statErr := errors.New("some stat error")
	*walk.LstatP = func(path string) (os.FileInfo, error) {
		if strings.HasSuffix(path, "stat-error") {
			return nil, statErr
		}
		return os.Lstat(path)
	}
	got := map[string]error{}
	err = walk.Walk(td, func(path string, fi os.FileInfo, hassub bool, err error) error {
		rel, _ := walk.Rel(td, path)
		mapmut.Lock()
		got[walk.ToSlash(rel)] = err // data race here, vs itself. add mapmut.
		mapmut.Unlock()
		return nil
	})
	if err != nil {
		t.Errorf("Walk error: %v", err)
	}
	want := map[string]error{
		".":              nil,
		"foo":            nil,
		"bar":            nil,
		"dir":            nil,
		"dir/baz":        nil,
		"dir/stat-error": statErr,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Walked %#v; want %#v", got, want)
	}
}

func TestBug3486(t *testing.T) { // http://code.google.com/p/go/issues/detail?id=3486
	// Skip this test under WASM since we don't have access to GOROOT
	if runtime.GOOS == "wasip1" || runtime.GOOS == "js" {
		t.Skip("Skipping TestBug3486 under WASM - no access to GOROOT")
	}

	root, err := walk.EvalSymlinks(runtime.GOROOT() + "/test")
	if err != nil {
		t.Fatal(err)
	}
	bugs := walk.Join(root, "bugs")
	ken := walk.Join(root, "ken")
	seenBugs := false
	seenKen := false
	walk.Walk(root, func(pth string, info os.FileInfo, hassub bool, err error) error {
		if err != nil {
			t.Fatal(err)
		}

		switch pth {
		case bugs:
			seenBugs = true
			return walk.SkipDir
		case ken:
			if !seenBugs {
				// now unsorted!
				//	t.Fatal("walk.Walk out of order - ken before bugs")
			}
			seenKen = true
		}
		return nil
	})
	if !seenKen {
		t.Fatalf("%q not seen", ken)
	}
}
