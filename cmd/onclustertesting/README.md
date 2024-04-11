# onclustertesting

**- What I did**

Provides a very simple binary for setting up / tearing down on-cluster builds to make testing / development go faster.

## Prerequisites
- An OpenShift 4.14 cluster
- Kubeconfig for the aforementioned cluster.
- Go toolchain
- Git
- _(optional, but recommended)_ [K9s](https://k9scli.io/)

## To use
1. Download the latest pre-built binary for your machine from the GitHub Releases.
1. Place the binary someplace in your `$PATH`.

You can now set up a very simple on-cluster build testing situation which makes
use of some handy defaults such as using the global pull secret and an
in-cluster OpenShift ImageStream for pushing the built image to.

To do this, run:
`$ ./onclustertesting setup in-cluster-registry` --enable-feature-gate --pool=layered

This will use the in-cluster registry which requires no external credentials to
be used. The pool will start building as soon as it can. Note: If the
`TechPreviewNoUpgrade` feature gate was not previously enabled, this will
create a new MachineConfig in all MachineConfigPools, incurring a full
MachineConfig rollout before the build will start.

You can then tear down everything created for this test by running:
`$ ./onclustertesting teardown`
