#!/usr/bin/env python3

import itertools
import os
import subprocess
import sys
import yaml

DOCKERFILE_NAME = "Dockerfile.goreleaser"


# See: https://github.com/yaml/pyyaml/issues/535
class VerboseSafeDumper(yaml.SafeDumper):
    def ignore_aliases(self, data):
        return True


def create_build(name):
    return {
        "main": f"./cmd/{name}",
        "id": name,
        "binary": name,
        "env": [
            "CGO_ENABLED=0",
        ],
        "ldflags": [
            "-s -w -X main.version={{.Version}} -X main.commit={{.Commit}} -X main.date={{.Date}} -X main.builtBy=goreleaser".replace(
                "main",
                "github.com/cheesesashimi/zacks-openshift-helpers/internal/pkg/version",
            ),
        ],
        "goos": [
            "darwin",
            "linux",
        ],
        "goarch": [
            "amd64",
            "arm64",
        ],
    }


# Removed deprecated create_docker_for_arch and create_docker_manifests_for_dockers functions.


def create_dockers_v2(builds):
    # This replaces the logic of create_docker_for_arch and create_docker_manifests_for_dockers.
    # It builds the consolidated dockers_v2 structure.
    ids = [build["id"] for build in builds]
    return [
        {
            "dockerfile": DOCKERFILE_NAME,
            "ids": ids,
            "images": [
                "quay.io/zzlotnik/{{ .ProjectName }}",
            ],
            "platforms": [
                "linux/amd64",
                "linux/arm64",
            ],
            "tags": [
                "{{ .Version }}",
                "latest",
            ],
            # Mapping from old build_flag_templates (labels) to new labels map format.
            "labels": {
                "org.opencontainers.image.title": "{{ .ProjectName }}",
                "org.opencontainers.image.description": "{{ .ProjectName }}",
                "org.opencontainers.image.url": "https://github.com/cheesesashimi/{{ .ProjectName }}",
                "org.opencontainers.image.source": "https://github.com/cheesesashimi/{{ .ProjectName }}",
                "org.opencontainers.image.version": "{{ .Version }}",
                # The time template needs to be converted to a label value.
                "org.opencontainers.image.created": "{{ .Date }}",
                "org.opencontainers.image.revision": "{{ .FullCommit }}",
                # GitHub Actions environment variable checks converted to label keys/values.
                '{{ if index .Env "GITHUB_ACTIONS" }}com.github.actions{{else}}label-no-actions-env-1{{end}}': "",
                '{{ if index .Env "GITHUB_RUN_ID" }}com.github.actions.runId{{else}}label-no-actions-env-2{{end}}': '{{ if index .Env "GITHUB_RUN_ID" }}{{ .Env.GITHUB_RUN_ID }}{{else}}{{end}}',
                '{{ if index .Env "GITHUB_RUN_NUMBER" }}com.github.actions.runNumber{{else}}label-no-actions-env-3{{end}}': '{{ if index .Env "GITHUB_RUN_NUMBER" }}{{ .Env.GITHUB_RUN_NUMBER }}{{else}}{{end}}',
                '{{ if index .Env "GITHUB_WORKFLOW" }}com.github.actions.workflow{{else}}label-no-actions-env-4{{end}}': '{{ if index .Env "GITHUB_WORKFLOW" }}{{ .Env.GITHUB_WORKFLOW }}{{else}}{{end}}',
                '{{ if index .Env "RUNNER_NAME" }}com.github.actions.runnerName{{else}}label-no-actions-env-5{{end}}': '{{ if index .Env "RUNNER_NAME" }}{{ .Env.RUNNER_NAME }}{{else}}{{end}}',
            },
        }
    ]


def update_goreleaser_file(names):
    goreleaser = ".goreleaser.yaml"

    with open(goreleaser, "r") as goreleaser_file:
        goreleaser_data = list(yaml.safe_load_all(goreleaser_file))[0]

    goreleaser_data["builds"] = [create_build(name) for name in names]
    goreleaser_data["builds"].sort(key=lambda x: x["id"])

    # Update with new dockers_v2 key and remove deprecated keys
    goreleaser_data["dockers_v2"] = create_dockers_v2(goreleaser_data["builds"])
    if "dockers" in goreleaser_data:
        del goreleaser_data["dockers"]
    if "docker_manifests" in goreleaser_data:
        del goreleaser_data["docker_manifests"]

    with open(goreleaser, "w") as goreleaser_file:
        yaml.dump(goreleaser_data, goreleaser_file, Dumper=VerboseSafeDumper)


def update_gitignore_file(names):
    gitignore = ".gitignore"
    with open(gitignore, "r") as gitignore_file:
        gitignore_content = gitignore_file.read()

    is_updated = False
    with open(gitignore, "a") as gitignore_file:
        for name in names:
            cmd_name = f"cmd/{name}/{name}"
            if cmd_name not in gitignore_content:
                is_updated = True
                print(f"Added {cmd_name} to {gitignore}")
                gitignore_file.write(
                    f"# Auto-added by {os.path.basename(__file__)} to ignore ad-hoc Go binaries\n{cmd_name}\n"
                )

    if not is_updated:
        print(f"{gitignore} already up-to-date")
    else:
        print(f"{gitignore} updated")


def update_dockerfile(names):
    print(f"Regenerating {DOCKERFILE_NAME}")

    dockerfile_lines = [
        "# DO NOT EDIT BY HAND!",
        "# Auto-generated by hack/update-configs.py",
        "",
    ]

    with open("Dockerfile.fetcher", "r") as fetcher:
        for line in fetcher:
            dockerfile_lines.append(line.rstrip())

    dockerfile_lines.append("")
    # Add ARG TARGETPLATFORM for use with dockers_v2 COPY [cite: 378, 980]
    dockerfile_lines.append("FROM quay.io/fedora/fedora:43 AS final")
    dockerfile_lines.append("ARG TARGETPLATFORM")
    dockerfile_lines.append("COPY --from=fetcher /oc/oc /usr/local/bin/oc")
    dockerfile_lines.append("COPY --from=fetcher /oc/kubectl /usr/local/bin/kubectl")

    # Update COPY commands to use $TARGETPLATFORM prefix as required by dockers_v2 [cite: 379, 982]
    for name in names:
        dockerfile_lines.append(f"COPY $TARGETPLATFORM/{name} /usr/local/bin/{name}")

    with open(DOCKERFILE_NAME, "w") as dockerfile:
        for line in dockerfile_lines:
            dockerfile.write(f"{line}\n")


def main():
    names = sorted(sys.argv[1:])

    ignored_names = frozenset(["mcdiff", "playground"])

    names = [name for name in names if name not in ignored_names]

    update_goreleaser_file(names)

    update_gitignore_file(names)

    update_dockerfile(names)


if __name__ == "__main__":
    main()
