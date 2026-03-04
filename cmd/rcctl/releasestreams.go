package main

import (
	"github.com/cheesesashimi/zacks-openshift-helpers/internal/pkg/releasecontroller"
	"github.com/spf13/cobra"
)

func releaseStreamsNamesCmd() *cobra.Command {
	namesCmd := &cobra.Command{
		Use:   "releases [releasestream]",
		Short: "List releasestreams and their releases.",
		Example: `
	# Lists all releases for all releasestreams.
	rcctl releasestreams releases all

	# Lists all releases for a given releasestream.
	rcctl releasestreams releases all '4-stable'

	# Lists all releases for multiple given releasestreams.
	rcctl releasestreams release all '4-stable' '5-stable'

	# Lists accepted releases for all releasestreams.
	rcctl releasestreams releases all

	# Lists accepted releases for a given releasestream.
	rcctl releasestreams releases all '4-stable'

	# Lists accepted releases for multiple given releasestreams.
	rcctl releasestreams release accepted '4-stable' '5-stable'

	# Lists rejected releases for all releasestreams.
	rcctl releasestreams releases all

	# Lists rejected releases for a given releasestream.
	rcctl releasestreams releases all '4-stable'

	# Lists rejected releases for multiple given releasestreams.
	rcctl releasestreams release rejected '4-stable' '5-stable'`,
	}

	cmds := []*cobra.Command{
		{
			Use:   "all [releasestream]",
			Short: "List all release names",
			RunE: func(cmd *cobra.Command, args []string) error {
				return doReleaseControllerOp(func(rc releasecontroller.ReleaseController) (interface{}, error) {
					return newReleaseStreamsHelper(rc).AllReleasesForReleaseStreams(args)
				})
			},
		},
		{
			Use:   "accepted [releasestream]",
			Short: "List only accepted release names",
			RunE: func(cmd *cobra.Command, args []string) error {
				return doReleaseControllerOp(func(rc releasecontroller.ReleaseController) (interface{}, error) {
					return newReleaseStreamsHelper(rc).AcceptedReleasesForReleaseStreams(args)
				})
			},
		},
		{
			Use:   "rejected [releasestream]",
			Short: "List only rejected release names",
			RunE: func(cmd *cobra.Command, args []string) error {
				return doReleaseControllerOp(func(rc releasecontroller.ReleaseController) (interface{}, error) {
					return newReleaseStreamsHelper(rc).RejectedReleasesForReleaseStreams(args)
				})
			},
		},
	}

	for _, cmd := range cmds {
		namesCmd.AddCommand(cmd)
	}

	return namesCmd
}

func releaseStreamCmd() *cobra.Command {
	rsCmd := &cobra.Command{
		Use:   "releasestreams",
		Short: "Query releasestreams",
	}

	rsListCmd := &cobra.Command{
		Use:   "list",
		Short: "List all releasestreams on the release controller",
		RunE: func(cmd *cobra.Command, args []string) error {
			return doReleaseControllerOp(func(rc releasecontroller.ReleaseController) (interface{}, error) {
				return newReleaseStreamsHelper(rc).AllReleaseStreamNames()
			})
		},
	}

	rsConfigCmd := &cobra.Command{
		Use:   "config [releasestream]",
		Short: "Shows the configuration for the given releasestream",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return doReleaseControllerOp(func(rc releasecontroller.ReleaseController) (interface{}, error) {
				return rc.ReleaseStream(args[0]).Config()
			})
		},
	}

	rsCmd.AddCommand(rsListCmd)
	rsCmd.AddCommand(releaseStreamsNamesCmd())
	rsCmd.AddCommand(rsConfigCmd)

	return rsCmd
}

func init() {
	rootCmd.AddCommand(releaseStreamCmd())
}
