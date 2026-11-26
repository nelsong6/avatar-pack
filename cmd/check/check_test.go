package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"image"
	"image/png"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func tinyPNG(t *testing.T, shade uint8) []byte {
	t.Helper()
	img := image.NewGray(image.Rect(0, 0, 2, 2))
	img.Pix[0] = shade
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// write lays out a pack in a new folder: each avatar's file, then pack.json from edit's
// changes to the entries.
func write(t *testing.T, files map[string][]byte, edit func(p map[string]any)) (string, []byte) {
	t.Helper()
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "originals"), 0o755)
	var avatars []any
	for _, id := range []string{"a1", "s1", "me"} {
		b, ok := files[id]
		if !ok {
			continue
		}
		os.WriteFile(filepath.Join(dir, "originals", id+".png"), b, 0o644)
		sum := sha256.Sum256(b)
		kind := map[string]string{"a1": "agent", "s1": "system", "me": "personal"}[id]
		handles := []any{}
		if kind == "agent" {
			handles = []any{"jim"}
		}
		avatars = append(avatars, map[string]any{"id": id, "kind": kind, "name": "N " + id, "handles": handles,
			"crop":     map[string]any{"size": 1, "center_x": 0.5, "center_y": 0.5},
			"original": "originals/" + id + ".png", "sha256": hex.EncodeToString(sum[:])})
	}
	p := map[string]any{"format": 1, "avatars": avatars}
	if edit != nil {
		edit(p)
	}
	b, _ := json.Marshal(p)
	os.WriteFile(filepath.Join(dir, "pack.json"), b, 0o644)
	return dir, b
}

func entry(p map[string]any, i int) map[string]any { return p["avatars"].([]any)[i].(map[string]any) }

func TestCheck(t *testing.T) {
	files := map[string][]byte{"a1": tinyPNG(t, 1), "s1": tinyPNG(t, 2), "me": tinyPNG(t, 3)}
	_, main := write(t, files, nil)

	cases := []struct {
		name  string
		files map[string][]byte
		edit  func(p map[string]any)
		extra func(dir string)
		want  string // "" is a good pack
	}{
		{name: "good", files: files},
		{name: "a removed avatar is allowed", files: map[string][]byte{"a1": files["a1"], "s1": files["s1"]}},
		{name: "no agent", files: map[string][]byte{"s1": files["s1"], "me": files["me"]}, want: "no agent avatar"},
		{name: "no system", files: map[string][]byte{"a1": files["a1"], "me": files["me"]}, want: "no system avatar"},
		{name: "a rename is allowed", files: files, edit: func(p map[string]any) { entry(p, 0)["name"] = "New"; entry(p, 0)["handles"] = []any{"joe"} }},
		{name: "format", files: files, edit: func(p map[string]any) { p["format"] = 2 }, want: "format must be 1"},
		{name: "unknown field", files: files, edit: func(p map[string]any) { entry(p, 0)["colour"] = "red" }, want: "unknown field"},
		{name: "kind", files: files, edit: func(p map[string]any) { entry(p, 1)["kind"] = "robot" }, want: "kind \"robot\""},
		{name: "two personal", files: files, edit: func(p map[string]any) { entry(p, 1)["kind"] = "personal" }, want: "2 personal"},
		{name: "a reviewer is allowed", files: files, edit: func(p map[string]any) { entry(p, 2)["kind"] = "reviewer" }},
		{name: "two reviewers", files: files, edit: func(p map[string]any) {
			entry(p, 2)["kind"] = "reviewer"
			second := maps.Clone(entry(p, 2))
			second["id"], second["original"] = "me2", "originals/me2.png"
			p["avatars"] = append(p["avatars"].([]any), second)
		}, extra: func(dir string) { os.WriteFile(filepath.Join(dir, "originals", "me2.png"), files["me"], 0o644) }, want: "2 reviewer"},
		{name: "agent without handle", files: files, edit: func(p map[string]any) { entry(p, 0)["handles"] = []any{} }, want: "at least one handle"},
		{name: "bad handle", files: files, edit: func(p map[string]any) { entry(p, 0)["handles"] = []any{"Jim"} }, want: "lowercase letters"},
		{name: "duplicate id", files: files, edit: func(p map[string]any) { entry(p, 1)["id"] = "a1"; entry(p, 1)["original"] = "originals/a1.png" }, want: "listed twice"},
		{name: "wrong sha", files: files, edit: func(p map[string]any) { entry(p, 0)["sha256"] = strings.Repeat("0", 64) }, want: "has sha256"},
		{name: "not a png", files: map[string][]byte{"a1": []byte("not a picture"), "s1": files["s1"]}, want: "does not decode as PNG"},
		{name: "missing file", files: files, extra: func(dir string) { os.Remove(filepath.Join(dir, "originals", "s1.png")) }, want: "s1.png"},
		{name: "stray file", files: files, extra: func(dir string) { os.WriteFile(filepath.Join(dir, "originals", "x.png"), nil, 0o644) }, want: "originals/x.png is not listed"},
		{name: "original path", files: files, edit: func(p map[string]any) { entry(p, 0)["original"] = "originals/s1.png" }, want: "original must be originals/a1.png"},
		{name: "crop not an object", files: files, edit: func(p map[string]any) { entry(p, 0)["crop"] = 1 }, want: "crop must be a JSON object"},
		{name: "changed picture", files: map[string][]byte{"a1": tinyPNG(t, 9), "s1": files["s1"], "me": files["me"]}, want: "a1: its picture changed"},
		{name: "changed crop", files: files, edit: func(p map[string]any) { entry(p, 0)["crop"] = map[string]any{"size": 0.5} }, want: "a1: its crop changed"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir, _ := write(t, c.files, c.edit)
			if c.extra != nil {
				c.extra(dir)
			}
			errs := check(dir, main)
			var all []string
			for _, e := range errs {
				all = append(all, e.Error())
			}
			got := strings.Join(all, "\n")
			if c.want == "" && got != "" || c.want != "" && !strings.Contains(got, c.want) {
				t.Fatalf("want %q, got:\n%s", c.want, got)
			}
		})
	}
}

// The crop is compared by meaning: main's own spacing and key order don't count as a change.
func TestCropKeyOrder(t *testing.T) {
	if !sameJSON([]byte(`{"size": 1, "center_x": 0.5}`), []byte(`{"center_x":0.5,"size":1}`)) {
		t.Fatal("same crop read as changed")
	}
}
