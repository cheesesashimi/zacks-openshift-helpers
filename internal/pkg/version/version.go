package version

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog"
)

var (
	version = "not-built-correctly"
	commit  = "not-built-correctly"
	date    = "not-built-correctly"
	builtBy = "not-built-correctly"
)

type moduleVersion struct {
	Name    string `json:"name"`
	Path    string `json:"-"`
	Version string `json:"version"`
}

func getModuleVersions() []moduleVersion {
	buildInfo, ok := debug.ReadBuildInfo()
	if !ok {
		klog.Warning("Unable to read debug info, some fields may be empty or missing.")
	}

	moduleVersions := []moduleVersion{}

	notableDeps := sets.Set[string]{}
	notableDeps.Insert(
		"k8s.io/api",
		"k8s.io/apimachinery",
		"k8s.io/client-go",
		"github.com/openshift/api",
		"github.com/openshift/client-go",
		"github.com/openshift/library-go",
		"github.com/openshift/machine-config-operator",
	)

	for _, dep := range buildInfo.Deps {
		if notableDeps.Has(dep.Path) {
			moduleVersions = append(moduleVersions, moduleVersion{
				Name:    dep.Path,
				Path:    dep.Path,
				Version: dep.Version,
			})
		}
	}

	return moduleVersions
}

func (m moduleVersion) String() string {
	return fmt.Sprintf("%s - %s", m.Name, m.Version)
}

type versionInfo struct {
	Name      string          `json:"name"`
	GoVersion string          `json:"goVersion"`
	Version   string          `json:"version"`
	Commit    string          `json:"commit"`
	Date      string          `json:"date"`
	BuiltBy   string          `json:"builtBy"`
	Modules   []moduleVersion `json:"modules"`
}

func newVersionInfo() (*versionInfo, error) {
	name, err := os.Executable()
	if err != nil {
		return nil, err
	}

	out := &versionInfo{
		Name:      filepath.Base(name),
		GoVersion: runtime.Version(),
		Version:   version,
		Commit:    commit,
		Date:      date,
		BuiltBy:   builtBy,
		Modules:   getModuleVersions(),
	}

	return out, nil
}

func (v versionInfo) String() string {
	sb := &strings.Builder{}

	fmt.Fprintf(sb, "%s\n", v.Name)
	fmt.Fprintf(sb, "Version: %s\n", v.Version)
	fmt.Fprintf(sb, "Commit: %s\n", v.Commit)
	fmt.Fprintf(sb, "Go Version: %s\n", v.GoVersion)
	fmt.Fprintf(sb, "Build Date: %s\n", v.Date)
	fmt.Fprintf(sb, "Built By: %s\n", v.BuiltBy)

	if len(v.Modules) != 0 {
		fmt.Fprintf(sb, "\nModule versions:\n")
		for _, module := range v.Modules {
			fmt.Fprintf(sb, "- %s\n", module)
		}
	}

	return sb.String()
}

func Command() *cobra.Command {
	jsonFormat := false

	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print the current version",
		RunE: func(_ *cobra.Command, _ []string) error {
			vi, err := newVersionInfo()
			if err != nil {
				return err
			}

			if jsonFormat {
				return printJSONVersionInfo(vi)
			}

			fmt.Println(vi)

			return nil
		},
	}

	cmd.PersistentFlags().BoolVar(&jsonFormat, "json", false, "Prints a JSON representation of the version info.")

	return cmd
}

func printJSONVersionInfo(vi *versionInfo) error {
	out, err := json.MarshalIndent(vi, "", "\t")
	if err != nil {
		return err
	}

	fmt.Println(string(out))
	return nil
}
