// check holds the pack to format 1's rules (README, The rules). With -base it also compares
// with the pack.json that git revision holds, so an id already there keeps its picture; a
// revision with no pack.json (before the first pack) has nothing to compare.
//
//	check [-base <git revision>] [pack folder]
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func main() {
	baseRev := flag.String("base", "", "git revision whose pack.json the pack is compared with")
	flag.Parse()
	dir := "."
	if flag.NArg() > 0 {
		dir = flag.Arg(0)
	}

	var base []byte
	if *baseRev != "" {
		var err error
		if base, err = packAt(dir, *baseRev); err != nil {
			fmt.Fprintln(os.Stderr, "check:", err)
			os.Exit(2)
		}
		if base == nil {
			fmt.Printf("%s has no pack.json: nothing to compare with\n", *baseRev)
		}
	}

	errs := check(dir, base)
	for _, err := range errs {
		fmt.Fprintln(os.Stderr, err)
	}
	if len(errs) > 0 {
		fmt.Fprintf(os.Stderr, "%d problems\n", len(errs))
		os.Exit(1)
	}
	if p, err := os.ReadFile(dir + "/pack.json"); err == nil {
		if pk, err := parse(p); err == nil {
			counts := map[string]int{}
			for _, a := range pk.Avatars {
				counts[a.Kind]++
			}
			fmt.Printf("pack is good: %d agent, %d system, %d personal, %d reviewer\n", counts["agent"], counts["system"], counts["personal"], counts["reviewer"])
		}
	}
}

// packAt reads pack.json at a git revision: nil when the revision has none, an error when
// the revision itself can't be read.
func packAt(dir, rev string) ([]byte, error) {
	if out, err := git(dir, "rev-parse", "--verify", "--quiet", rev+"^{commit}"); err != nil {
		return nil, fmt.Errorf("can't read revision %s: %v %s", rev, err, out)
	}
	if _, err := git(dir, "cat-file", "-e", rev+":pack.json"); err != nil {
		return nil, nil
	}
	out, err := git(dir, "show", rev+":pack.json")
	if err != nil {
		return nil, fmt.Errorf("git show %s:pack.json: %v", rev, err)
	}
	return out, nil
}

func git(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return out, fmt.Errorf("%s", strings.TrimSpace(stderr.String()))
		}
	}
	return out, err
}
