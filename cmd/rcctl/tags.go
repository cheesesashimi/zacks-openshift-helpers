package main

import (
	"github.com/cheesesashimi/zacks-openshift-helpers/internal/pkg/releasecontroller"
	"github.com/spf13/cobra"
)

func tagsCmd() *cobra.Command {
	tagsCmd := &cobra.Command{
		Use:   "tags [releasestream]",
		Short: "View tags for a releasestream",
		Args:  cobra.ExactArgs(1),
	}

	cmds := []*cobra.Command{
		{
			Use:   "all [releasestream]",
			Short: "Show all tags",
			Example: `
	# Shows all tags for a given releasestream
	rcctl tags all '4.23.0-0.ci'`,
			Args: cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				return doReleaseControllerOp(func(rc releasecontroller.ReleaseController) (interface{}, error) {
					return getTagsByPhase(rc, "", args[0])
				})
			},
		},
		{
			Use:   "accepted [releasestream]",
			Short: "Show accepted tags",
			Example: `
	# Shows all accepted tags for a given releasestream
	rcctl tags accepted '4.23.0-0.ci'`,
			Args: cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				return doReleaseControllerOp(func(rc releasecontroller.ReleaseController) (interface{}, error) {
					return getTagsByPhase(rc, releasecontroller.PhaseAccepted, args[0])
				})
			},
		},
		{
			Use:   "ready [releasestream]",
			Short: "Show ready tags",
			Example: `
	# Shows all ready tags for a given releasestream
	rcctl tags ready '4.23.0-0.ci'`,
			Args: cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				return doReleaseControllerOp(func(rc releasecontroller.ReleaseController) (interface{}, error) {
					return getTagsByPhase(rc, releasecontroller.PhaseReady, args[0])
				})
			},
		},
		{
			Use:   "latest [releasestream]",
			Short: "Get the latest accepted tag",
			Example: `
	# Shows the latest accepted tag for a given releasestream
	rcctl tags latest '4.23.0-0.ci'`,
			Args: cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				return doReleaseControllerOp(func(rc releasecontroller.ReleaseController) (interface{}, error) {
					return rc.ReleaseStream(args[0]).Latest()
				})
			},
		},
		{
			Use:   "rejected [releasestream]",
			Short: "Show rejected tags",
			Example: `
	# Shows all rejected tags for a given releasestream
	rcctl tags rejected '4.23.0-0.ci'`,
			Args: cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				return doReleaseControllerOp(func(rc releasecontroller.ReleaseController) (interface{}, error) {
					return getTagsByPhase(rc, releasecontroller.PhaseRejected, args[0])
				})
			},
		},
	}

	for _, cmd := range cmds {
		tagsCmd.AddCommand(cmd)
	}

	return tagsCmd
}

func getTagsByPhase(rc releasecontroller.ReleaseController, phase releasecontroller.Phase, releaseStream string) (*releasecontroller.ReleaseTags, error) {
	if phase == "" {
		return rc.ReleaseStream(releaseStream).Tags()
	}

	return rc.ReleaseStream(releaseStream).TagsByPhase(phase)
}

func init() {
	rootCmd.AddCommand(tagsCmd())
}
