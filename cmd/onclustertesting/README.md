# onclustertesting

**- What I did**

Provides a very simple binary for setting up / tearing down on-cluster builds to make testing / development go faster.

## Prerequisites
- An OpenShift 4.14 cluster
- Kubeconfig for the aforementioned cluster.
- Go toolchain
- Git
- _(optional, but recommended)_ [Official GitHub CLI](https://cli.github.com/)
- _(optional, but recommended)_ [K9s](https://k9scli.io/)

## To use
1. Check out this PR locally. The [official GitHub CLI](https://cli.github.com/) makes this very easy if you have a local clone of this repo: `$ gh pr checkout 3852`.
1. Change to the binary directory: `$ cd cmd/onclustertesting`
1. Build the binary: `$ go build .`
1. Place the built binary somewhere in your `PATH` or run it directly from the directory you built it in.

You can now set up a very simple on-cluster build testing situation which makes use of some handy defaults such as using the global pull secret and an in-cluster OpenShift ImageStream for pushing the built image to.

To do this, run:
`$ ./onclustertesting setup in-cluster-registry`

You can then tear down everything created for this test by running:
`$ ./onclustertesting teardown`

## Known Limitations
- I did my best to fill in the CLI options for help, but I recognize that there will be gaps.
- There are places where I hard-coded paths on my system. You may need to adjust those for your system and rebuild the binary.
