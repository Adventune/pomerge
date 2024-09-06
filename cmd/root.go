package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/adventune/pomerge/pomerge"
)

var (
	rootCmd = &cobra.Command{
		Use:   "pomerge a b c [out]",
		Short: "3-way merging tool for .po-files",
		Long: `pomerge is a 3-way merging tool for .po-files.
It is intended to be used as a merge driver for git.

Install the driver with:
    git config merge.merge-po-files.driver "pomerge %A %O %B"

Note: Git attributes must be set up to use the driver.
Add the following line to .gitattributes in the repository:
    [attr]POFILE merge=merge-po-files
    *.po POFILE`,
		Args: validateArgs,
		Run:  run,
	}
	verbose bool
)

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
}

// Run the command
func run(cmd *cobra.Command, args []string) {
	a := args[0]
	b := args[1]
	c := args[2]
	out := a // default output file is the same as the first input file
	if len(args) == 4 {
		out = args[3]
	}
	err := pomerge.ThreeWayOut(a, b, c, out, verbose)
	if err != nil {
		exitErr(err.Error())
	}
}

// Exit with an error message
func exitErr(msg string) {
	fmt.Println(msg)
	os.Exit(1)
}

// Validates arguments
func validateArgs(cmd *cobra.Command, args []string) error {
	// Minimum and maximum number of arguments
	if err := cobra.MinimumNArgs(3)(cmd, args); err != nil {
		return err
	}
	if err := cobra.MaximumNArgs(4)(cmd, args); err != nil {
		return err
	}

	// Check if the files exist (except for the output file)
	for i, arg := range args {
		if i == 3 {
			break
		}
		if _, err := os.Stat(arg); os.IsNotExist(err) {
			return err
		}
	}

	return nil
}
