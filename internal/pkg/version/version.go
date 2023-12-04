package version

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func Command(version, commit, buildDate string) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the current version",
		RunE: func(_ *cobra.Command, _ []string) error {
			name, err := os.Executable()
			if err != nil {
				return err
			}

			fmt.Println(filepath.Base(name))
			fmt.Printf("Version: %s\n", version)
			fmt.Printf("Commit: %s\n", commit)
			fmt.Printf("Build Date: %s\n", buildDate)

			return nil
		},
	}
}
