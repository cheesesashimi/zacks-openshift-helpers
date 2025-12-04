# dualstream-release-builder

This is intended to be used to create a dualstream (both RHEL9 and RHEL10) release payload image for OCP:

```console
dualstream-release-builder \
  --base-release "registry.ci.openshift.org/ocp/release:4.22.0-0.ci-2025-12-04-004014" \
  --rhel10-pullspec "registry.ci.openshift.org/rhcos-devel/node-staging@sha256:857318f7a12783c71bda0d4e7c8eab3dcadfe88fb7a8ab937abc5592eaac640d" \
  --rhel10-ext-pullspec "registry.ci.openshift.org/rhcos-devel/node-staging@sha256:8959dae45e038084c115e23815fab775177ba97071ccee4f080bd4859060bee5" \
  --final-release-pullspec "quay.io/zzlotnik/testing:dualstream-release-payload" \
  --src-authfile "/path/to/src/creds/authfile" \
  --dst-authfile "/path/to/dst/creds/authfile"
```

It will use a combination of `podman`, `skopeo`, and `oc` to do the following:
1. Fetch the given base release payload image.
2. Add the RHEL10 and RHEL10 extensions images to the list of tags while including certain values from their image labels.
3. Update all mentions of the release version by appending the string `-dualstream` onto it.
4. Builds and pushes a release payload image.

The end result is this:
```console
$ oc adm release info quay.io/zzlotnik/testing:dualstream-release-payload | grep "coreos"
  rhel-coreos                                    sha256:51c88549776c59bfd49e12c358cf32071f6131750883dee8666f2fc3d878e8d4
  rhel-coreos-extensions                         sha256:24aa147cd8f9358fe4100ed248ffb63d663f0a855dbf7a87f656d4edaa09638e
  rhel-coreos-10                                 sha256:857318f7a12783c71bda0d4e7c8eab3dcadfe88fb7a8ab937abc5592eaac640d
  rhel-coreos-10-extensions                      sha256:8959dae45e038084c115e23815fab775177ba97071ccee4f080bd4859060bee5

$ oc adm release info -o=json quay.io/zzlotnik/testing:dualstream-release-payload | jq -r '[.references.spec.tags[] | select(.name | contains("coreos"))]'
[
  {
    "name": "rhel-coreos",
    "annotations": {
      "io.openshift.build.commit.id": "2b6ac09e306e4b2fa293a887ef88196bfe6c3143",
      "io.openshift.build.commit.ref": "",
      "io.openshift.build.source-location": "https://github.com/openshift/os",
      "io.openshift.build.version-display-names": "machine-os=Red Hat Enterprise Linux CoreOS",
      "io.openshift.build.versions": "machine-os=9.6.20251201-1",
      "io.openshift.os.streamclass": "rhel-9"
    },
    "from": {
      "kind": "DockerImage",
      "name": "registry.ci.openshift.org/ocp/4.22-2025-12-04-004014@sha256:51c88549776c59bfd49e12c358cf32071f6131750883dee8666f2fc3d878e8d4"
    },
    "generation": null,
    "importPolicy": {},
    "referencePolicy": {
      "type": ""
    }
  },
  {
    "name": "rhel-coreos-extensions",
    "annotations": {
      "io.openshift.build.commit.id": "2b6ac09e306e4b2fa293a887ef88196bfe6c3143",
      "io.openshift.build.commit.ref": "",
      "io.openshift.build.source-location": "https://github.com/openshift/os"
    },
    "from": {
      "kind": "DockerImage",
      "name": "registry.ci.openshift.org/ocp/4.22-2025-12-04-004014@sha256:24aa147cd8f9358fe4100ed248ffb63d663f0a855dbf7a87f656d4edaa09638e"
    },
    "generation": null,
    "importPolicy": {},
    "referencePolicy": {
      "type": ""
    }
  },
  {
    "name": "rhel-coreos-10",
    "annotations": {
      "io.openshift.build.commit.id": "2b6ac09e306e4b2fa293a887ef88196bfe6c3143",
      "io.openshift.build.commit.ref": "",
      "io.openshift.build.source-location": "https://github.com/openshift/os",
      "io.openshift.build.version-display-names": "machine-os=Red Hat Enterprise Linux CoreOS",
      "io.openshift.build.versions": "machine-os=10.1.20251202-0",
      "io.openshift.os.streamclass": "rhel-10"
    },
    "from": {
      "kind": "DockerImage",
      "name": "registry.ci.openshift.org/rhcos-devel/node-staging@sha256:857318f7a12783c71bda0d4e7c8eab3dcadfe88fb7a8ab937abc5592eaac640d"
    },
    "generation": null,
    "importPolicy": {},
    "referencePolicy": {
      "type": ""
    }
  },
  {
    "name": "rhel-coreos-10-extensions",
    "annotations": {
      "io.openshift.build.commit.id": "2b6ac09e306e4b2fa293a887ef88196bfe6c3143",
      "io.openshift.build.commit.ref": "",
      "io.openshift.build.source-location": "https://github.com/openshift/os"
    },
    "from": {
      "kind": "DockerImage",
      "name": "registry.ci.openshift.org/rhcos-devel/node-staging@sha256:8959dae45e038084c115e23815fab775177ba97071ccee4f080bd4859060bee5"
    },
    "generation": null,
    "importPolicy": {},
    "referencePolicy": {
      "type": ""
    }
  }
]
```
