# cluster-lifecycle

This is a simple program intended to help bring up sandbox OpenShift clusters. It is essentially a translation of my scripts [found here](https://github.com/cheesesashimi/oc-oneliners/tree/main/cluster-lifecycle) into Golang for easy portability and reuse.

## To Use:

1. Download a prebuilt binary for your operating system / architecture.
2. To bring up an OpenShift cluster, you can call the program directly or you can put everything into a script:
    ```shell
    #!/usr/bin/env bash

    set -xeuo

    cluster-lifecycle setup \
            --prefix "your-username" \
            --release-arch "amd64" \
            --release-stream "4.14.0-0.ci" \
            --ssh-key-path "/path/to/your/public/ssh/keys" \
            --pull-secret-path "/path/to/your/pull/secret" \
            --work-dir "$HOME/.openshift-installer"
    ```
3. To tear down your OpenShift cluster, run the program directly or put everything into a script:
    ```shell
    #!/usr/bin/env bash

    cluster-lifecycle teardown --work-dir "$HOME/.openshift-installer"
    ```

## How It Works:
1. It validates your choice of cluster kind. Current supported kinds are "ocp", "okd", "okd-scos".
2. It validates your choice of cluster architecture. Current supported arches by kind are:
    ```yaml
    ocp:
    - amd64
    - arm64
    - multi

    okd:
    - amd64

    okd-scos:
    - amd64
    ```
3. Using this information, it reaches out to the appropriate release controller to get the latest release for the given release stream.
4. It downloads and extracts the appropriate `openshift-install` binary from the given release.
5. It writes a simple `install-config.yaml` to the working directory, using the provided prefix, cluster kind, and cluster arch to generate the name, e.g.: `zzlotnik-ocp-amd64`.
6. It calls `openshift-install` within the working directory to bring up the cluster.

## Using a preexisting `install-config.yaml`

`cluster-lifecycle` now supports using a preexisting `install-config.yaml` file. When a path to such a file is provided, it will read the contents of the config file and perform the following actions:

1. Copy the file into the work directory specified. This is done because `openshift-install` will delete the `install-config.yaml` file once its contents have been consumed. The original file will be left unmodified. It will not allow you to keep the `install-config.yaml` file in your work directory for this reason.
2. Generate and override the cluster name within the file if the `--prefix` flag is set. The name will follow the same convention above of `<prefix>-<kind>-<arch>`. However, the architecture value will be read from the `controlPlane` section of the `install-config.yaml` file.
3. Read the SSH key and pull secret files and inject them into the `install-config.yaml` file, overwriting any values present.
4. If the `--enable-tech-preview` flag is used, it will add `TechPreviewNoUpgrade` to the file.

```console
cluster-lifecycle setup \
        --enable-tech-preview \
        --install-config "/path/to/your/install-config.yaml" \
        --prefix "mycluster" \
        --release-stream "4.22.0-0.ci" \
        --release-kind ocp \
        --ssh-key-path "/path/to/your/ssh/key" \
        --pull-secret-path "/path/to/your/pull-secret" \
        --work-dir "/path/to/your/desired-workdir"
```

## Additional Features
- By adding a `.vacation` file to the working directory, the program will skip cluster setup.
- By adding a `.release` file to the working directory containing a release pullspec, the program will always bring up that release.

## Limitations
- Currently only supports AWS.