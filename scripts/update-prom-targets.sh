#!/usr/bin/env bash
# HW6 Part III: keep Prometheus pointed at every running ECS task.
# Every 15 s, look up the public IP of each RUNNING task in the service
# and write them to a Prometheus file_sd targets file. Prometheus re-reads
# that file automatically, so tasks added or removed by auto scaling (or
# stopped in resilience tests) appear/disappear without restarts.
#
# Run from the repo root and leave it running in its own terminal:
#   ./scripts/update-prom-targets.sh
#
# The file is written OUTSIDE the repo (~/cs6650-prom) because the repo
# lives on the iCloud-synced Desktop, which Docker can't reliably mount.
set -uo pipefail

cd "$(dirname "$0")/../terraform"
CLUSTER=$(terraform output -raw ecs_cluster_name)
SERVICE=$(terraform output -raw ecs_service_name)

OUT_DIR="$HOME/cs6650-prom"
OUT_FILE="$OUT_DIR/targets.json"
mkdir -p "$OUT_DIR"

while true; do
  IPS=""
  TASKS=$(aws ecs list-tasks --cluster "$CLUSTER" --service-name "$SERVICE" \
    --desired-status RUNNING --query 'taskArns' --output text 2>/dev/null)

  if [ -n "$TASKS" ] && [ "$TASKS" != "None" ]; then
    # shellcheck disable=SC2086  (word-splitting the ARN list is intended)
    ENIS=$(aws ecs describe-tasks --cluster "$CLUSTER" --tasks $TASKS \
      --query "tasks[].attachments[].details[?name=='networkInterfaceId'].value[]" \
      --output text 2>/dev/null)
    if [ -n "$ENIS" ] && [ "$ENIS" != "None" ]; then
      # shellcheck disable=SC2086
      IPS=$(aws ec2 describe-network-interfaces --network-interface-ids $ENIS \
        --query 'NetworkInterfaces[].Association.PublicIp' --output text 2>/dev/null)
    fi
  fi

  TARGETS=""
  for ip in $IPS; do
    [ "$ip" = "None" ] && continue   # task still starting, no public IP yet
    TARGETS="${TARGETS}\"${ip}:8080\","
  done
  TARGETS="[${TARGETS%,}]"

  # Write atomically so Prometheus never reads a half-written file
  echo "[{\"targets\": ${TARGETS}}]" > "$OUT_FILE.tmp"
  mv "$OUT_FILE.tmp" "$OUT_FILE"

  echo "$(date +%H:%M:%S)  scraping: ${TARGETS}"
  sleep 15
done
