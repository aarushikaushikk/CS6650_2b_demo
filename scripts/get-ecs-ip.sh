#!/usr/bin/env bash
# Prints the public IP of the first running task in the ECS service.
# Run from the repo root; reads cluster/service names from Terraform outputs.
set -euo pipefail

cd "$(dirname "$0")/../terraform"
CLUSTER=$(terraform output -raw ecs_cluster_name)
SERVICE=$(terraform output -raw ecs_service_name)

TASK=$(aws ecs list-tasks --cluster "$CLUSTER" --service-name "$SERVICE" \
  --desired-status RUNNING --query 'taskArns[0]' --output text)
if [ -z "$TASK" ] || [ "$TASK" = "None" ]; then
  echo "No running task found in $SERVICE" >&2
  exit 1
fi

ENI=$(aws ecs describe-tasks --cluster "$CLUSTER" --tasks "$TASK" \
  --query "tasks[0].attachments[0].details[?name=='networkInterfaceId'].value" \
  --output text)

aws ec2 describe-network-interfaces --network-interface-ids "$ENI" \
  --query 'NetworkInterfaces[0].Association.PublicIp' --output text