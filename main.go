package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/rickypai/bdiff/bazel"
	"github.com/rickypai/bdiff/changes"
	"github.com/rickypai/bdiff/cmd"
	"github.com/rickypai/bdiff/filesystem"
)

var (
	debugFlag     = flag.Bool("debug", false, "debug")
	bazelBinFlag  = flag.String("bazel-bin", "bazel", "bazel-bin")
	baseRefFlag   = flag.String("base", "", "base ref")
	targetRefFlag = flag.String("target", "HEAD", "target ref")
	testsOnlyFlag = flag.Bool("tests-only", false, "debug")
)

func main() {
	flag.Parse()

	var bazelBin string
	if bazelBinFlag != nil {
		bazelBin = *bazelBinFlag
	} else {
		bazelBin = "bazelisk"
	}

	var debug bool
	if *debugFlag {
		debug = true
	}

	var baseRef string
	if baseRefFlag != nil {
		baseRef = *baseRefFlag
	} else {
		baseRef = ""
	}

	var targetRef string
	if targetRefFlag != nil {
		targetRef = *targetRefFlag
	} else {
		targetRef = ""
	}

	var testsOnly bool
	if *testsOnlyFlag {
		testsOnly = true
	}

	dir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	if debug {
		fmt.Println(dir)
	}

	// TODO: check workspace status and use that if dirty
	// TODO: git args
	out, stderr, err := cmd.ExecCommand(dir, "git", "diff", "--name-only", fmt.Sprintf("%s..%s", baseRef, targetRef))
	if err != nil {
		println(out)
		println(stderr)
		log.Fatal(err)
	}

	allFiles := strings.Split(string(out), "\n")
	srcFiles := make([]string, 0, len(allFiles))
	delFiles := make([]string, 0, len(allFiles))

	var targets []string

	for _, file := range allFiles {
		if debug {
			println(file)
		}

		if len(file) < 1 {
			continue
		}

		if file == "WORKSPACE" {
			// assume everything changed if WORKSPACE changed
			targets = []string{"//..."}
			break
		}

		// ignore deleted files for now
		if !filesystem.FileExists(file) {
			delFiles = append(delFiles, file)
			continue
		}

		buildTargets := changes.BuildFileChanges(file)

		if len(buildTargets) > 0 {
			targets = append(targets, buildTargets...)
			continue
		}

		// check if file is tracked by Bazel
		tracked, stderr, err := bazel.FileTracked(dir, bazelBin, file, debug)
		if err != nil {
			println(stderr)
			log.Fatal(err)
		}

		// don't bother with files not tracked by Bazel
		if !tracked {
			continue
		}

		srcFiles = append(srcFiles, file)
	}

	if len(srcFiles) > 0 {
		rdeps := "rdeps(//..., set(" + strings.Join(srcFiles, " ") + "))"
		foundTargets, stderr, err := bazel.Query(dir, bazelBin, rdeps, debug)
		if err != nil {
			println(stderr)
			log.Fatal(err)
		}

		for _, foundTarget := range foundTargets {
			if strings.Contains(foundTarget, "~") {
				continue
			}

			targets = append(targets, foundTarget)
		}
	}

	var set string

	if testsOnly {
		set = "tests(set(" + strings.Join(targets, " ") + "))"
	} else {
		set = "set(" + strings.Join(targets, " ") + ")"
	}

	finalTargets, stderr, err := bazel.Query(dir, bazelBin, set, debug)
	if err != nil {
		println(stderr)
		log.Fatal(err)
	}

	for _, target := range finalTargets {
		fmt.Println(target)
	}
}
