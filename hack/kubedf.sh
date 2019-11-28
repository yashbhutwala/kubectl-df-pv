#!/usr/bin/env bash

# Pre-requisites: have "kubectl proxy" running
# Dependencies: 'bash', 'curl', 'jq', 'awk', 'sed'
# Modified from: https://gist.github.com/redmcg/60cfff7bca6f32969188008ad4a44c9a


# https://news.ycombinator.com/item?id=10736584
set -o errexit -o nounset -o pipefail
# set -o errexit -o nounset -o pipefail -o xtrace
# this line enables debugging
#set -xv


KUBEAPI=127.0.0.1:8001/api/v1/nodes

function getNodes() {
  curl -s $KUBEAPI | jq -r '.items[].metadata.name'
}

function getPVCs() {
#  this works
#  curl -f $KUBEAPI/worker0-yashb-2019-10-28-ci-kube1-15-3-1m4w/proxy/stats/summary | jq -s '[flatten | .[].pods[] | select(.volume[] | has("pvcRef")) | ''{podName: .podRef.name, namespaceName: .podRef.namespace, pvcName: .volume[].pvcRef.name}] | sort_by(.pvcName)'
  jq -r -s '[flatten | .[].pods[] | select(.volume[]? | has("pvcRef")) | ''{podName: .podRef.name, namespaceName: .podRef.namespace, ''pvcName: (.volume[] | select(has("pvcRef")) | .pvcRef.name), ''capacityBytes: (.volume[] | select(has("pvcRef")) | .capacityBytes), ''usedBytes: (.volume[] | select(has("pvcRef")) | .usedBytes), ''availableBytes: (.volume[] | select(has("pvcRef")) | .availableBytes), ''percentageUsed: (.volume[] | select(has("pvcRef")) | (.usedBytes / .capacityBytes * 100)), ''iused: (.volume[] | select(has("pvcRef")) | .inodesUsed), ''ifree: (.volume[] | select(has("pvcRef")) | .inodesFree), ''iusedPercentage: (.volume[] | select(has("pvcRef")) | (.inodesUsed / .inodes * 100))}] | sort_by(.pvcName)'
}

function column() {
  awk '{ for (i = 1; i <= NF; i++) { d[NR, i] = $i; w[i] = length($i) > w[i] ? length($i) : w[i] } } ''END { for (i = 1; i <= NR; i++) { printf("%-*s", w[1], d[i, 1]); for (j = 2; j <= NF; j++ ) { printf("%*s", w[j] + 1, d[i, j]) } print "" } }'
}

function defaultFormat() {
  awk 'BEGIN { print "PVC Namespace Pod 1024-blocks Used Available %Used iused ifree %iused" } ''{$4 = $4/1024; $5 = $5/1024; $6 = $6/1024; $7 = sprintf("%.2f%%",$7); $10 = sprintf("%.2f%%",$10); print $0}'
}

function humanFormat() {
  awk 'BEGIN { print "PVC Namespace Pod Size Used Available \%Used iused ifree \%iused" } ''{$7 = sprintf("%.2f%%",$7); $10 = sprintf("%.2f%%",$10); printf("%s ", $1); printf("%s ", $2); printf("%s ", $3); system(sprintf("numfmt --to=iec-i %s %s %s | sed '\''N;N;s/\\n/ /g'\'' | tr -d \\\\n", $4, $5, $6)); printf(" %s ", $7); system(sprintf("numfmt --to=iec-i %s %s | sed '\''N;s/\\n/ /g'\'' | tr -d \\\\n", $8, $9)); print " " $10}'
}

function format() {
  jq -r '.[] | "\(.pvcName) \(.namespaceName) \(.podName) \(.capacityBytes) \(.usedBytes) \(.availableBytes) \(.percentageUsed) \(.iused) \(.ifree) \(.iusedPercentage)"' |
    sed 's/^"\|"$//g' |
    $format | column
}

desiredFormat=${1:-k}
if [ "$desiredFormat" == "-h" ]; then
  format=humanFormat
else
  format=defaultFormat
fi

for node in $(getNodes); do
  curl -s "$KUBEAPI/$node/proxy/stats/summary"
done | getPVCs | format
