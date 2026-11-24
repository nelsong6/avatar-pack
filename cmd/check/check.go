package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"unicode/utf8"
)

// avatar is one entry of pack.json, format 1 (README, Format 1).
type avatar struct {
	ID       string          `json:"id"`
	Kind     string          `json:"kind"`
	Name     string          `json:"name"`
	Handles  []string        `json:"handles"`
	Crop     json.RawMessage `json:"crop"`
	Original string          `json:"original"`
	SHA256   string          `json:"sha256"`
}

type pack struct {
	Format  *int     `json:"format"`
	Avatars []avatar `json:"avatars"`
}

var (
	idShape     = regexp.MustCompile(`^[A-Za-z0-9_-]{1,100}$`)
	handleShape = regexp.MustCompile(`^[a-z0-9]+$`)
	shaShape    = regexp.MustCompile(`^[0-9a-f]{64}$`)
	kinds       = map[string]bool{"agent": true, "system": true, "personal": true}
)

// parse reads pack.json strictly: an unknown field is a mistake, not something to skip.
func parse(b []byte) (pack, error) {
	var p pack
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err := d.Decode(&p); err != nil {
		return p, fmt.Errorf("pack.json: %w", err)
	}
	if d.More() {
		return p, fmt.Errorf("pack.json: more than one JSON value")
	}
	return p, nil
}

// check holds the pack in dir to every rule, and, with base (main's pack.json before the
// change) not nil, to the rule that an id's picture never changes. It returns every
// problem found, not just the first.
func check(dir string, base []byte) []error {
	var errs []error
	fail := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }

	b, err := os.ReadFile(filepath.Join(dir, "pack.json"))
	if err != nil {
		return []error{err}
	}
	p, err := parse(b)
	if err != nil {
		return []error{err}
	}
	if p.Format == nil || *p.Format != 1 {
		return []error{fmt.Errorf("pack.json: format must be 1")}
	}

	seen := map[string]bool{}
	listed := map[string]bool{}
	personal := 0
	for i, a := range p.Avatars {
		at := fmt.Sprintf("avatar %d (%s)", i, a.ID)
		if !idShape.MatchString(a.ID) {
			fail("%s: id must be 1 to 100 letters, digits, - and _ (it goes in a URL)", at)
		}
		if seen[a.ID] {
			fail("%s: id is listed twice", at)
		}
		seen[a.ID] = true
		if !kinds[a.Kind] {
			fail("%s: kind %q is not agent, system or personal", at, a.Kind)
		}
		if a.Kind == "personal" {
			personal++
		}
		if n := utf8.RuneCountInString(a.Name); n < 1 || n > 80 {
			fail("%s: name must be 1 to 80 characters", at)
		}
		for _, h := range a.Handles {
			if !handleShape.MatchString(h) {
				fail("%s: handle %q must be lowercase letters and digits", at, h)
			}
		}
		if a.Kind == "agent" && len(a.Handles) == 0 {
			fail("%s: an agent needs at least one handle", at)
		}
		var crop map[string]any
		if err := json.Unmarshal(a.Crop, &crop); err != nil || crop == nil {
			fail("%s: crop must be a JSON object", at)
		}
		want := "originals/" + a.ID + ".png"
		if a.Original != want {
			fail("%s: original must be %s", at, want)
			continue
		}
		listed[a.ID+".png"] = true
		if !shaShape.MatchString(a.SHA256) {
			fail("%s: sha256 must be 64 lowercase hex digits", at)
		}
		f, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(a.Original)))
		if err != nil {
			fail("%s: %v", at, err)
			continue
		}
		if sum := sha256.Sum256(f); hex.EncodeToString(sum[:]) != a.SHA256 {
			fail("%s: %s has sha256 %x, not the %s listed", at, a.Original, sum, a.SHA256)
		}
		if _, err := png.Decode(bytes.NewReader(f)); err != nil {
			fail("%s: %s does not decode as PNG: %v", at, a.Original, err)
		}
	}
	if personal > 1 {
		fail("pack.json: %d personal avatars, at most one", personal)
	}
	// A service refuses a pack with none of a dealt kind: loading it would mark every
	// avatar of that kind it holds deleted (nelsong6/avatars#30).
	for _, kind := range []string{"agent", "system"} {
		if !hasKind(p.Avatars, kind) {
			fail("pack.json: no %s avatar, and a pack needs at least one", kind)
		}
	}

	entries, err := os.ReadDir(filepath.Join(dir, "originals"))
	if err != nil && !os.IsNotExist(err) {
		fail("originals: %v", err)
	}
	var stray []string
	for _, e := range entries {
		if !listed[e.Name()] {
			stray = append(stray, "originals/"+e.Name())
		}
	}
	sort.Strings(stray)
	for _, s := range stray {
		fail("%s is not listed in pack.json: remove it with its entry", s)
	}

	if base != nil {
		old, err := parse(base)
		if err != nil {
			fail("main's %v", err)
			return errs
		}
		now := map[string]avatar{}
		for _, a := range p.Avatars {
			now[a.ID] = a
		}
		for _, o := range old.Avatars {
			a, ok := now[o.ID]
			if !ok {
				continue // removed, which is allowed
			}
			if a.SHA256 != o.SHA256 {
				fail("%s: its picture changed (sha256 %s on main): a changed picture is a new id", o.ID, o.SHA256)
			}
			if !sameJSON(a.Crop, o.Crop) {
				fail("%s: its crop changed (%s on main), which changes its circle: a re-cropped picture is a new id", o.ID, o.Crop)
			}
		}
	}
	return errs
}

func hasKind(avatars []avatar, kind string) bool {
	for _, a := range avatars {
		if a.Kind == kind {
			return true
		}
	}
	return false
}

// sameJSON compares two JSON values by meaning, not by spacing or key order.
func sameJSON(a, b json.RawMessage) bool {
	var x, y any
	if json.Unmarshal(a, &x) != nil || json.Unmarshal(b, &y) != nil {
		return bytes.Equal(a, b)
	}
	return reflect.DeepEqual(x, y)
}
