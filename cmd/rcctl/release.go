package main

import (
	"fmt"

	"github.com/cheesesashimi/zacks-openshift-helpers/internal/pkg/releasecontroller"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/sets"
)

func releaseCmd() *cobra.Command {
	releaseCmd := &cobra.Command{
		Use:   "release",
		Short: "Operations on a specific release",
	}

	var allComponentMetadata bool
	var components []string

	ocInfoCmd := &cobra.Command{
		Use:   "oc-info [tag name]",
		Short: "Retrieves release info using 'oc adm release info'.",
		Example: `
	# Gets the release info for a release image pullspec.
	rcctl release oc-info 'quay.io/openshift-release-dev/ocp-release:4.21.4-x86_64'

	# Gets the release info for a release tag.
	rcctl release oc-info '4.21.4-x86_64'

	# Gets the release info and retrieves component image metadata for all component images.
	rcctl release oc-info '4.21.4-x86_64' --all-components

	# Gets the release info and retrieves component image metadata only for the provided component images.
	rcctl release oc-info '4.21.4-x86_64' --component 'machine-config-operator' --component 'rhel-coreos'

	# Gets the release info and retrieves component image metadata only for the provided component images (comma-separated).
	rcctl release oc-info '4.21.4-x86_64' --component 'machine-config-operator,rhel-coreos'`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if allComponentMetadata && len(components) != 0 {
				return fmt.Errorf("--all cannot be combined with --component")
			}

			return doReleaseControllerOp(func(rc releasecontroller.ReleaseController) (interface{}, error) {
				rif := releasecontroller.NewReleaseInfoFetcher(rc)

				if allComponentMetadata {
					return rif.FetchWithAllComponents(args[0])
				}

				if len(components) == 0 {
					return rif.FetchReleaseInfo(args[0])
				}

				dedupedComponents := sets.New[string](components...).UnsortedList()
				return rif.FetchWithComponents(args[0], dedupedComponents)
			})
		},
	}

	ocInfoCmd.PersistentFlags().StringSliceVar(&components, "component", []string{}, "Component(s) metadata to fetch.")
	ocInfoCmd.PersistentFlags().BoolVar(&allComponentMetadata, "all-components", false, "Fetches all component image metadata.")

	infoCmd := &cobra.Command{
		Use:   "info [tag name]",
		Short: "Retrieves release info from the release controller",
		Args:  cobra.ExactArgs(1),
		Example: `
	# Gets release info from the release controller for a given release tag.
	rcctl release info '4.21.4-x86_64'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return doReleaseControllerOp(func(rc releasecontroller.ReleaseController) (interface{}, error) {
				rc, err := getReleaseController()
				if err != nil {
					return nil, err
				}

				stream, release, err := rc.ReleaseStreams().FindReleaseNameAndStream(args[0])
				if err != nil {
					return nil, err
				}

				return rc.ReleaseStream(stream).Tag(release)
			})
		},
	}

	releaseCmd.AddCommand(ocInfoCmd)
	releaseCmd.AddCommand(infoCmd)

	return releaseCmd
}

func init() {
	rootCmd.AddCommand(releaseCmd())
}
