#!/usr/bin/env bash
# CRAP gate over go-crap reports.
#
# Usage: hack/crap-gate.sh report|check|baseline
#
# Environment (required ones are set by the Makefile; the rest have defaults and can be
# overridden, e.g. make crap-check CRAP_THRESHOLD=20):
#   GO_CRAP, GOJQ            tool binaries (required)
#   CRAP_PROFILE             coverage profile written by go test [coverage.out]
#   CRAP_BASELINE            committed baseline [.crap-baseline.json]
#   CRAP_REPORT              JSON report of the last check [crap.json]
#   CRAP_THRESHOLD           score above which a new function fails [30]
#   CRAP_REGRESSION_DELTA    score increase above which an existing function fails [1.0]
#   CRAP_EXCLUDES            space-separated regexes of files to skip [_gen\.go$]
# A "$" in a value passed as a make argument is written "$$".
#
# Exit codes: 0 pass, 1 usage or tool error, 2 gate failure.
set -euo pipefail

: "${GO_CRAP:?}" "${GOJQ:?}"
: "${CRAP_PROFILE:=coverage.out}"
: "${CRAP_BASELINE:=.crap-baseline.json}"
: "${CRAP_REPORT:=crap.json}"
: "${CRAP_THRESHOLD:=30}"
: "${CRAP_REGRESSION_DELTA:=1.0}"
: "${CRAP_EXCLUDES=_gen\.go\$}"

# Functions are matched to the baseline by file and name; absent from the baseline means new.
# $t is a jq variable.
# shellcheck disable=SC2016
readonly NEW_FUNCTIONS='
  .entries[]
  | select((.baseline_crap // 0) == 0 and .crap > $t)
  | "  \(.file):\(.line) \(.function): \(.crap * 100 | round / 100) (CC \(.cyclomatic), coverage \(.coverage | round)%)"'

# Baseline keeps file, function and scores rounded to 0.1, sorted; effective_crap is the compared value.
readonly BASELINE_FORMAT='
  def r: if . == null then null else . * 10 | round / 10 end;
  {entries: ([.entries[]
    | {file, function, cyclomatic,
       coverage: (.coverage | r), crap: (.crap | r), effective_crap: (.effective_crap | r)}]
    | sort_by(.file, .function))}'

scan() {
  local -a args excludes
  args=(scan ./... --coverage-profile "$CRAP_PROFILE" --threshold "$CRAP_THRESHOLD" --no-progress)
  read -r -a excludes <<<"$CRAP_EXCLUDES"
  local re
  # The +-expansion keeps an empty array valid under set -u in bash 3.2 (macOS /bin/bash).
  for re in ${excludes[@]+"${excludes[@]}"}; do
    args+=(--exclude "$re")
  done
  "$GO_CRAP" "${args[@]}" "$@"
}

report() {
  scan --top 20
}

check() {
  if [[ ! -f $CRAP_BASELINE ]]; then
    scan --top 20
    echo "WARNING: $CRAP_BASELINE not found, CRAP gate skipped: run make crap-baseline" >&2
    return 0
  fi

  local failed=0 new
  rm -f "$CRAP_REPORT"
  scan --baseline "$CRAP_BASELINE" --fail-regression \
    --fail-regression-threshold "$CRAP_REGRESSION_DELTA" -f json -o "$CRAP_REPORT" || failed=1
  if [[ ! -s $CRAP_REPORT ]]; then
    echo "go-crap produced no report" >&2
    return 1
  fi

  new=$("$GOJQ" -r --argjson t "$CRAP_THRESHOLD" "$NEW_FUNCTIONS" "$CRAP_REPORT")
  if [[ -n $new ]]; then
    printf 'CRAP new functions above %s:\n%s\n' "$CRAP_THRESHOLD" "$new" >&2
    failed=1
  fi

  if ((failed)); then
    echo "CRAP gate failed, details in $CRAP_REPORT: add tests or simplify the listed functions; make crap-baseline accepts an intended change" >&2
    return 2
  fi
  echo "CRAP gate passed"
}

baseline() {
  scan -f json -o "$CRAP_REPORT"
  "$GOJQ" "$BASELINE_FORMAT" "$CRAP_REPORT" >"$CRAP_BASELINE"
  echo "CRAP baseline written to $CRAP_BASELINE"
}

main() {
  if [[ ! -f $CRAP_PROFILE ]]; then
    echo "$CRAP_PROFILE not found: run make test first" >&2
    return 1
  fi

  case "${1:-}" in
  report | check | baseline) "$1" ;;
  *)
    echo "usage: $0 report|check|baseline" >&2
    return 1
    ;;
  esac
}

main "$@"
