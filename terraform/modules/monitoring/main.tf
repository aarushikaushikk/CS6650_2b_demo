# HW6 Part III: CloudWatch dashboard with the metrics the assignment
# asks for, so they can be watched together during a scaling event.

locals {
  ecs_dims = ["ClusterName", var.cluster_name, "ServiceName", var.service_name]
  alb_dims = ["LoadBalancer", var.alb_arn_suffix]
  tg_dims  = ["TargetGroup", var.target_group_arn_suffix, "LoadBalancer", var.alb_arn_suffix]

  widget = {
    type   = "metric"
    width  = 12
    height = 6
  }
}

resource "aws_cloudwatch_dashboard" "this" {
  dashboard_name = "${var.service_name}-scaling"

  dashboard_body = jsonencode({
    widgets = [
      merge(local.widget, {
        x = 0, y = 0
        properties = {
          title   = "ECS CPU utilization (avg / min / max across tasks)"
          region  = var.region
          view    = "timeSeries"
          period  = 60
          metrics = [
            concat(["AWS/ECS", "CPUUtilization"], local.ecs_dims, [{ stat = "Average", label = "Average" }]),
            concat(["AWS/ECS", "CPUUtilization"], local.ecs_dims, [{ stat = "Minimum", label = "Minimum" }]),
            concat(["AWS/ECS", "CPUUtilization"], local.ecs_dims, [{ stat = "Maximum", label = "Maximum" }]),
          ]
          annotations = { horizontal = [{ label = "Scaling target", value = var.cpu_target }] }
        }
      }),
      merge(local.widget, {
        x = 12, y = 0
        properties = {
          title   = "Desired vs running task count"
          region  = var.region
          view    = "timeSeries"
          period  = 60
          stat    = "Average"
          metrics = [
            concat(["ECS/ContainerInsights", "DesiredTaskCount"], local.ecs_dims),
            concat(["ECS/ContainerInsights", "RunningTaskCount"], local.ecs_dims),
          ]
        }
      }),
      merge(local.widget, {
        x = 0, y = 6
        properties = {
          title   = "ALB request count (per minute)"
          region  = var.region
          view    = "timeSeries"
          period  = 60
          metrics = [
            concat(["AWS/ApplicationELB", "RequestCount"], local.alb_dims, [{ stat = "Sum", label = "Total requests" }]),
            concat(["AWS/ApplicationELB", "RequestCountPerTarget"], local.tg_dims, [{ stat = "Sum", label = "Requests per target" }]),
          ]
        }
      }),
      merge(local.widget, {
        x = 12, y = 6
        properties = {
          title   = "ALB target response time (seconds)"
          region  = var.region
          view    = "timeSeries"
          period  = 60
          metrics = [
            concat(["AWS/ApplicationELB", "TargetResponseTime"], local.alb_dims, [{ stat = "Average", label = "Average" }]),
            concat(["AWS/ApplicationELB", "TargetResponseTime"], local.alb_dims, [{ stat = "p95", label = "p95" }]),
          ]
        }
      }),
      merge(local.widget, {
        x = 0, y = 12
        properties = {
          title   = "Target group healthy / unhealthy targets"
          region  = var.region
          view    = "timeSeries"
          period  = 60
          stat    = "Minimum"
          metrics = [
            concat(["AWS/ApplicationELB", "HealthyHostCount"], local.tg_dims),
            concat(["AWS/ApplicationELB", "UnHealthyHostCount"], local.tg_dims),
          ]
        }
      }),
      merge(local.widget, {
        x = 12, y = 12
        properties = {
          title   = "5xx errors (target vs ALB) and memory %"
          region  = var.region
          view    = "timeSeries"
          period  = 60
          metrics = [
            concat(["AWS/ApplicationELB", "HTTPCode_Target_5XX_Count"], local.alb_dims, [{ stat = "Sum", label = "Target 5xx" }]),
            concat(["AWS/ApplicationELB", "HTTPCode_ELB_5XX_Count"], local.alb_dims, [{ stat = "Sum", label = "ALB 5xx (e.g. no healthy targets)" }]),
            concat(["AWS/ECS", "MemoryUtilization"], local.ecs_dims, [{ stat = "Maximum", label = "Memory % (max)", yAxis = "right" }]),
          ]
        }
      }),
    ]
  })
}
