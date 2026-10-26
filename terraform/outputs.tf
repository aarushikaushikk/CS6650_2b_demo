output "ecs_cluster_name" {
  description = "Name of the created ECS cluster"
  value       = module.ecs.cluster_name
}

output "ecs_service_name" {
  description = "Name of the running ECS service"
  value       = module.ecs.service_name
}

# HW6 Part III: use this as the Locust host (http://<value>)
output "alb_dns_name" {
  description = "Public DNS name of the Application Load Balancer"
  value       = module.alb.alb_dns_name
}

output "cloudwatch_dashboard" {
  description = "CloudWatch dashboard with CPU, task counts, ALB metrics"
  value       = module.monitoring.dashboard_name
}
