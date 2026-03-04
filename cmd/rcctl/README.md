# rcctl

This is a simple CLI tool that can query the OpenShift Release Controller API and return useful information about the various releases contained therein.

The intent of this CLI tool is that it will be used as part of scripts and
other automation which rely on querying the release controller. Therefore, all
data returned from it will be returned as JSON to stdout.

```console
rcctl --help

The intent of this CLI tool is that it will be used as part of scripts and
other automation which rely on querying the release controller. Therefore, all
data returned from it will be returned as JSON to stdout.

Usage:
  rcctl [command]

Available Commands:
  completion     Generate the autocompletion script for the specified shell
  help           Help about any command
  release        Operations on a specific release
  releasestreams Query releasestreams
  tags           View tags for a releasestream

Flags:
      --controller string   Override the default release controller (default "amd64.ocp.releases.ci.openshift.org")
  -h, --help                help for rcctl

Use "rcctl [command] --help" for more information about a command.
```

## Examples

### Viewing what release streams are on a given release controller

```console
$ rcctl releasestreams list
[
    "4-dev-preview",
    "4-stable",
    // ...
]
```

### Viewing the release tags for a given releasestream

```console
$ rcctl releasestreams releases accepted '4-stable'
{
    "4-stable": [
        "4.21.4",
        "4.21.3",
        "4.21.2",
        "4.21.1",
        "4.21.0",
        // ...
    ]
}
```

### Getting the latest release for a given releasestream

```console
$ rcctl tags latest '4-stable'
{
    "name": "4.21.4",
    "phase": "Accepted",
    "pullSpec": "quay.io/openshift-release-dev/ocp-release:4.21.4-x86_64",
}
```

### Getting info about a given release tag or image pullspec including release component image metadata (requires `oc` and `skopeo`)

```console
$ rcctl release oc-info '4.23.0-0.ci-2026-03-05-153752' --component 'machine-config-operator,rhel-coreos'
{
  "releaseInfo": {
    "image": "registry.ci.openshift.org/ocp/release:4.23.0-0.ci-2026-03-05-153752",
    "digest": "sha256:aa6cd007e204673ceafa266fe1cf359b386cbb1e34c785ae2dd1856e8f61b71c",
    "contentDigest": "sha256:aa6cd007e204673ceafa266fe1cf359b386cbb1e34c785ae2dd1856e8f61b71c",
    "listDigest": "sha256:51ae51980f0c8c9c3b0e8a7af7948bcfa9b0356ce1a34cab4732db19e4934a1d"
    // ...
  },
  "componentMetadata": {
    "machine-config-operator": {
      "Name": "registry.ci.openshift.org/ocp/4.23-2026-03-05-153752",
      "Digest": "sha256:bec41abb841b042589766901962cf99bb7894bd673d4f71602523aa0c255a4f4",
      "RepoTags": [],
      "Created": "2026-03-05T06:02:19.899228885Z"
      // ...
    },
    "rhel-coreos": {
      "Name": "registry.ci.openshift.org/ocp/4.23-2026-03-05-153752",
      "Digest": "sha256:eccbe17a07f73e67689e2617855525c81de69fcb06f188b29b46c69c95c92242",
      "RepoTags": [],
      "Created": "2026-02-26T08:17:48.992044737Z"
      // ...
    }
  }
}
```
