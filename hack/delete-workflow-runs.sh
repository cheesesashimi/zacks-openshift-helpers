#!/usr/bin/env bash

owner="cheesesashimi"

for repo in $(gh repo list cheesesashimi --source --no-archived --json name | jq -r '.[].name'); do
  for run_id in $(gh run list --repo "$owner/$repo" -L50 --json databaseId | jq -r '.[].databaseId'); do
    echo "Deleting /repos/$owner/$repo/actions/runs/$run_id"

    gh api \
      --method DELETE \
      -H "Accept: application/vnd.github+json" \
      -H "X-GitHub-Api-Version: 2022-11-28" \
      "/repos/$owner/$repo/actions/runs/$run_id"
  done
done
