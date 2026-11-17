#!/bin/sh
# release-notes.sh <tag> — print that release's section of CHANGELOG.md,
# without its heading. Fails if the section is missing or empty, so a release
# is never published without saying what it does.
set -eu
tag=${1:?usage: release-notes.sh <tag>}
notes=$(awk -v t="## $tag " 'index($0, t) == 1 {f = 1; next} /^## / {f = 0} f' CHANGELOG.md |
  sed -e '/./,$!d')
[ -n "$(printf '%s' "$notes" | tr -d '[:space:]')" ] || { echo "release-notes: no CHANGELOG section for $tag" >&2; exit 1; }
printf '%s\n' "$notes"
