#!/bin/sh
# run.sh <workflow dir> [parallel] [ids...]: every problem (or the ids given), N at a time; results/<workflow>.tsv.
set -u
B=$(cd "$(dirname "$0")" && pwd); WFDIR=$1; PAR=${2:-3}; [ $# -ge 2 ] && shift 2 || shift
ids=${*:-$(ls "${PROBLEMS:-$B/problems}")}
printf '%s\n' $ids | xargs -P "$PAR" -I{} "$B/run-one.sh" "$WFDIR" {}
WF=$(basename "$WFDIR")
awk -F'\t' '{n++; split($4,a,"/"); if (a[1]!="" && a[1]==a[2]) p++} END {print "--- " p+0 " of " n " solved on all tests"}' "$B/results/$WF.tsv"
