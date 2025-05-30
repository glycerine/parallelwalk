// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pwalk

import (
	//"errors"
	"strings"
)

// IsAbs returns true if the path is absolute.
// In WASM/WASI, absolute paths start with "/"
func IsAbs(path string) bool {
	return strings.HasPrefix(path, "/")
}

// volumeNameLen returns length of the leading volume name on Windows.
// It returns 0 elsewhere.
func volumeNameLen(path string) int {
	return 0
}

// HasPrefix exists for historical compatibility and should not be used.
func HasPrefix(p, prefix string) bool {
	return strings.HasPrefix(p, prefix)
}

func splitList(path string) []string {
	if path == "" {
		return []string{}
	}
	return strings.Split(path, string(ListSeparator))
}

/*
// Clean returns the shortest path name equivalent to path
// by purely lexical processing. It applies the following rules
// iteratively until no further processing can be done:
//
//  1. Replace multiple Separator elements with a single one.
//  2. Eliminate each . path name element (the current directory).
//  3. Eliminate each inner .. path name element (the parent directory)
//     along with the non-.. element that precedes it.
//  4. Eliminate .. elements that begin a rooted path:
//     that is, replace "/.." by "/" at the beginning of a path,
//     assuming Separator is '/'.
//
// The returned path ends in a slash only if it represents a root directory.
//
// If the result of this process is an empty string, Clean
// returns the string ".".
func Clean(path string) string {
	// WASM/WASI paths are always forward slashes
	path = strings.ReplaceAll(path, "\\", "/")

	// Handle empty path
	if path == "" {
		return "."
	}

	// Handle absolute path
	rooted := strings.HasPrefix(path, "/")
	path = strings.TrimPrefix(path, "/")

	// Split into components
	parts := strings.Split(path, "/")

	// Process components
	var result []string
	for _, part := range parts {
		switch part {
		case "", ".":
			// Skip empty parts and current directory
			continue
		case "..":
			// Handle parent directory
			if len(result) > 0 {
				result = result[:len(result)-1]
			} else if !rooted {
				// Only append .. if not rooted and we can't go up
				result = append(result, "..")
			}
		default:
			result = append(result, part)
		}
	}

	// Reconstruct path
	if len(result) == 0 {
		if rooted {
			return "/"
		}
		return "."
	}

	path = strings.Join(result, "/")
	if rooted {
		path = "/" + path
	}
	return path
}

// Join joins any number of path elements into a single path,
// separating them with slashes. Empty elements are ignored.
// The result is Cleaned.
func Join(elem ...string) string {
	for i, e := range elem {
		if e != "" {
			return Clean(strings.Join(elem[i:], "/"))
		}
	}
	return ""
}

// Split splits path immediately following the final Separator,
// separating it into a directory and file name component.
// If there is no Separator in path, Split returns an empty dir
// and file set to path.
func Split(path string) (dir, file string) {
	i := strings.LastIndex(path, "/")
	if i == -1 {
		return "", path
	}
	return path[:i], path[i+1:]
}

// VolumeName returns the leading volume name.
// For WASM/WASI, this is always empty.
func VolumeName(path string) string {
	return ""
}

// Rel returns a relative path that is lexically equivalent to targpath when
// joined to basepath with an intervening separator.
func Rel(basepath, targpath string) (string, error) {
	base := Clean(basepath)
	targ := Clean(targpath)

	if targ == base {
		return ".", nil
	}

	// Handle absolute paths
	baseRooted := strings.HasPrefix(base, "/")
	targRooted := strings.HasPrefix(targ, "/")
	if baseRooted != targRooted {
		return "", errors.New("Rel: can't make " + targ + " relative to " + base)
	}

	// Remove leading slashes for comparison
	base = strings.TrimPrefix(base, "/")
	targ = strings.TrimPrefix(targ, "/")

	// Split into components
	baseParts := strings.Split(base, "/")
	targParts := strings.Split(targ, "/")

	// Find common prefix
	i := 0
	for i < len(baseParts) && i < len(targParts) && baseParts[i] == targParts[i] {
		i++
	}

	// Build relative path
	var result []string
	for j := i; j < len(baseParts); j++ {
		result = append(result, "..")
	}
	result = append(result, targParts[i:]...)

	if len(result) == 0 {
		return ".", nil
	}
	return strings.Join(result, "/"), nil
}
*/
