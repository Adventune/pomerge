package pomerge

import (
	"fmt"
	"os"
	"os/exec"

	"golang.org/x/sync/errgroup"
)

var verbose bool

/*
Original sh implementation translated to Go from https://stackoverflow.com/a/68799310/11764989

Extracted from the shell script credits:
    # Three-way merge driver for PO files, runs on multiple CPUs where possible
    #
    # Copyright 2015-2016 Marco Ciampa
    # Copyright 2021 Mikko Rantalainen <mikko.rantalainen@iki.fi>
    # License: MIT (https://opensource.org/licenses/MIT)
    #
    # Original source:
    # https://stackoverflow.com/a/29535676/334451
    # https://github.com/mezis/git-whistles/blob/master/libexec/git-merge-po.sh
*/

// Performs a 3-way merge of the files a, b, and c, and writes the result to a.
func ThreeWay(a, b, c string, v bool) error {
	return ThreeWayOut(a, b, c, a, v)
}

// Performs a 3-way merge of the files a, b, and c, and writes the result to out.
func ThreeWayOut(local, base, other, out string, v bool) error {
	verbose = v
	// Check dependencies
	err := checkDependencies()
	if err != nil {
		return err
	}

	// Make temporary directory
	tmpDir, err := os.MkdirTemp("/tmp", "pomerge-")
	if err != nil {
		return fmt.Errorf("failed to create temporary directory: %s", err)
	}
	defer os.RemoveAll(tmpDir)
	status(fmt.Sprintf("created temporary directory: %s", tmpDir))

	// Merge files
	status("beginning merge ...")

	// ======= Extract the header from the local file
	err = runExecutable("msggrep", "--force-po", "-N", "//", "-o", tmpDir+"/header", local)
	if err != nil {
		return fmt.Errorf("failed to extract header from local file: %s", err)
	}

	// ======= Clean input files and use logical filenames for possible conflict markers:
	status("canonicalizing input files ...")
	var waitgroup errgroup.Group
	waitgroup.Go(func() error {
		return cleanInput(base, tmpDir)
	})
	waitgroup.Go(func() error {
		return cleanInput(local, tmpDir)
	})
	waitgroup.Go(func() error {
		return cleanInput(other, tmpDir)
	})
	if err := waitgroup.Wait(); err != nil {
		return err
	}

	// ======= Extract changes and unchanged messages
	status("computing local-changes, other-changes and unchanged ...")
	// Extract changes in local and other
	waitgroup.Go(func() error {
		return extractChanges(tmpDir+"/local", tmpDir+"/base", tmpDir+"/local-changes")
	})
	waitgroup.Go(func() error {
		return extractChanges(tmpDir+"/other", tmpDir+"/base", tmpDir+"/other-changes")
	})
	// Extract unchanged messages
	waitgroup.Go(func() error {
		return extractUnchanged(tmpDir)
	})
	if err := waitgroup.Wait(); err != nil {
		return err
	}

	// ======= Compute conflicts
	status("computing conflicts ...")
	cmd1 := exec.Command(
		"msgcat",
		"--force-po",
		"-o",
		"-",
		tmpDir+"/other-changes",
		tmpDir+"/local-changes",
	)
	cmd2 := exec.Command("msggrep", "--msgstr", "-F", "-e", "'#-#-#-#-#'", "-")
	cmd3 := exec.Command("tee", tmpDir+"/conflicts")
	err = runPipeline(cmd1, cmd2, cmd3)
	if err != nil {
		return err
	}

	// ======= Messages changed on local, not on other; and vice-versa:
	status("computing local-only and other-only changes ...")
	waitgroup.Go(func() error {
		return localOnly(tmpDir+"/local-changes", tmpDir+"/conflicts", tmpDir+"/local-only")
	})
	waitgroup.Go(func() error {
		return localOnly(tmpDir+"/other-changes", tmpDir+"/conflicts", tmpDir+"/other-only")
	})
	if err := waitgroup.Wait(); err != nil {
		return err
	}

	// Note: following steps need to be done in sequence

	status("computing initial merge without template ...")
	err = runExecutable(
		"msgcat",
		"--force-po",
		"-o",
		tmpDir+"/merge1",
		tmpDir+"/unchanged",
		tmpDir+"/conflicts",
		tmpDir+"/local-only",
		tmpDir+"/other-only",
	)
	if err != nil {
		return err
	}

	// create a template to only output messages that are actually needed (union of messages on local and other create the template!)
	status("computing template and applying it to merge result ...")
	cmd1 = exec.Command(
		"msgcat",
		"--force-po",
		"-o",
		"-",
		tmpDir+"/local",
		tmpDir+"/other",
	)
	cmd2 = exec.Command(
		"msgmerge",
		"--force-po",
		"--quiet",
		"--no-fuzzy-matching",
		"-o",
		tmpDir+"/merge2",
		tmpDir+"/merge1",
		"-",
	)
	err = runPipeline(cmd1, cmd2)
	if err != nil {
		return err
	}

	// ======= Fix the header after merge
	status("fixing the header after merge ...")
	err = runExecutable(
		"msgcat",
		"--force-po",
		"--no-wrap",
		"--sort-output",
		"-o",
		tmpDir+"/merge3",
		"--use-first",
		tmpDir+"/header",
		tmpDir+"/merge2",
	)
	if err != nil {
		return err
	}

	// ======= Produce output file (overwrites input LOCAL file because git expects that for the results)
	status("saving output ...")
	err = os.Rename(tmpDir+"/merge3", out)
	if err != nil {
		return err
	}

	status("checking for conflicts in the result ...")
	err = runExecutable("grep", "-q", "'#-#-#-#-#'", out)
	if err != nil {
		if verbose {
			fmt.Println("conflict(s) detected")
		}
		return fmt.Errorf("automatic merge failed, exiting with status 1")
	} else {
		status("automatic merge completed successfully, exiting with status 0")
	}

	return nil
}

//// File manipulation function

// Changes only in the local file and not in the other file.
func localOnly(localChanges string, conflicts string, out string) error {
	return runExecutable("msgcat", "--force-po", "-o", out, "--unique", localChanges, conflicts)
}

// Cleans input files and uses logical filenames for possible conflict markers.
func cleanInput(file string, tmpDir string) error {
	return runExecutable("msguniq", "--force-po", "-o", tmpDir+"/"+file, "--unique", file)
}

// Extract unchanged
func extractUnchanged(tmpDir string) error {
	cmd1 := exec.Command(
		"msgcat",
		"--force-po",
		"-o",
		"-",
		tmpDir+"/base",
		tmpDir+"/local",
		tmpDir+"/other",
	)
	cmd2 := exec.Command("msggrep", "-v", "--msgstr", "-F", "-e", "'#-#-#-#-#'", "-")
	cmd3 := exec.Command("tee", tmpDir+"/unchanged")
	return runPipeline(cmd1, cmd2, cmd3)
}

// Extract changes
func extractChanges(in string, base string, out string) error {
	cmd1 := exec.Command("msgcat", "-o", "-", in, base)
	cmd2 := exec.Command("msggrep", "--msgstr", "-F", "-e", "'#-#-#-#-#'", "-")
	cmd3 := exec.Command(
		"msgmerge",
		"--force-po",
		"--quiet",
		"--no-fuzzy-matching",
		"-o",
		"-",
		in,
		"-",
	)
	cmd4 := exec.Command("msgattrib", "--no-obsolete")
	cmd5 := exec.Command("tee", out)

	// Run the pipeline
	return runPipeline(cmd1, cmd2, cmd3, cmd4, cmd5)
}

//// Command execution functions

// Runs an executable with the given arguments.
func runExecutable(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Runs a pipeline of commands.
// stdout of each command is connected to the stdin of the next command.
func runPipeline(cmds ...*exec.Cmd) error {
	for i := 0; i < len(cmds)-1; i++ {
		out, err := cmds[i].StdoutPipe()
		if err != nil {
			return err
		}
		cmds[i+1].Stdin = out
	}

	cmds[len(cmds)-1].Stdout = os.Stdout
	for _, cmd := range cmds {
		if err := cmd.Start(); err != nil {
			return err
		}
	}

	for _, cmd := range cmds {
		if err := cmd.Wait(); err != nil {
			return err
		}
	}

	return nil
}

//// Utility functions

// Checks that all "gettext" dependencies are installed.
func checkDependencies() error {
	// List of all executables that are required
	executables := []string{
		"msgmerge", "msgattrib", "msggrep", "msgcat", "msguniq", "grep",
	}

	// Loop through each executable and check if it's available
	for _, exe := range executables {
		if _, err := exec.LookPath(exe); err != nil {
			return fmt.Errorf("executable '%s' needs to be installed.\n", exe)
		}
	}
	return nil
}

// Prints a status message if verbose output is enabled.
func status(msg string) {
	if verbose {
		fmt.Println(msg)
	}
}
