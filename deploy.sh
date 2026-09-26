#!/usr/bin/env bash
# Deploy a tag (default: the newest one) of this repo and bring the stack up.
#
# Untracked/ignored files -- .env and the runtime state under config/ -- are
# never touched by `git checkout`, so they survive a deploy as-is. The only
# thing that can block a deploy is a locally edited *tracked* file, which is
# refused rather than silently overwritten.
#
# Usage: ./deploy.sh [tag-or-commit]
set -euo pipefail

cd "$(dirname "$(readlink -f "$0")")"

[[ -f .env ]] || { echo "deploy: no .env here -- copy .env.example and fill it in first" >&2; exit 1; }

if ! git diff --quiet HEAD --; then
    echo "deploy: tracked files have local edits, refusing to deploy over them:" >&2
    git status --short --untracked-files=no >&2
    exit 1
fi

git fetch --tags --prune --quiet origin

ref=${1:-$(git tag --sort=-version:refname | head -n1)}
[[ -n $ref ]] || { echo "deploy: no tags to deploy -- pass a tag or commit" >&2; exit 1; }

from=$(git describe --tags --always)
git -c advice.detachedHead=false checkout --quiet --detach "$ref"
echo "deploy: $from -> $(git describe --tags --always)"

docker compose up -d --build --remove-orphans
docker compose ps --format 'table {{.Service}}\t{{.State}}\t{{.Status}}'
