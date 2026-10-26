# ECS Cluster
resource "aws_ecs_cluster" "this" {
  name = "${var.service_name}-cluster"

  # HW6 Part III: publishes DesiredTaskCount / RunningTaskCount metrics
  # (ECS/ContainerInsights) for the "desired vs running" view.
  setting {
    name  = "containerInsights"
    value = "enabled"
  }
}

# Task Definition
resource "aws_ecs_task_definition" "this" {
  family                   = "${var.service_name}-task"
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]
  cpu                      = var.cpu
  memory                   = var.memory

  execution_role_arn = var.execution_role_arn
  task_role_arn      = var.task_role_arn

  container_definitions = jsonencode([{
    name      = "${var.service_name}-container"
    image     = var.image
    essential = true

    portMappings = [{
      containerPort = var.container_port
    }]

    logConfiguration = {
      logDriver = "awslogs"
      options = {
        "awslogs-group"         = var.log_group_name
        "awslogs-region"        = var.region
        "awslogs-stream-prefix" = "ecs"
      }
    }
  }])
}

# ECS Service
resource "aws_ecs_service" "this" {
  name            = var.service_name
  cluster         = aws_ecs_cluster.this.id
  task_definition = aws_ecs_task_definition.this.arn
  desired_count   = var.ecs_count
  launch_type     = "FARGATE"

  network_configuration {
    subnets          = var.subnet_ids
    security_groups  = var.security_group_ids
    assign_public_ip = true
  }

  # HW6 Part III: register every task with the ALB target group.
  load_balancer {
    target_group_arn = var.target_group_arn
    container_name   = "${var.service_name}-container"
    container_port   = var.container_port
  }

  # Give a new task time to seed 100,000 products before ALB health
  # check failures count against it.
  health_check_grace_period_seconds = var.health_check_grace_period

  # The autoscaler owns desired_count after creation; without this,
  # every terraform apply would reset it back to ecs_count.
  lifecycle {
    ignore_changes = [desired_count]
  }
}
