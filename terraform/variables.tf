# Region to deploy into
variable "aws_region" {
  type    = string
  default = "us-west-2"
}

# ECR & ECS settings
variable "ecr_repository_name" {
  type    = string
  default = "ecr_service"
}

variable "service_name" {
  type    = string
  default = "CS6650L2"
}

variable "container_port" {
  type    = number
  default = 8080
}

# Initial task count when the service is created (HW6 Part III: start at
# the autoscaling minimum). After that the autoscaler controls it.
variable "ecs_count" {
  type    = number
  default = 2
}

# How long to keep logs
variable "log_retention_days" {
  type    = number
  default = 7
}

# ---------- HW6 Part III: ALB ----------
variable "deregistration_delay" {
  type        = number
  default     = 30
  description = "Seconds the ALB drains a target before removing it (AWS default 300)"
}

# ---------- HW6 Part III: auto scaling (assignment's sample policy) ----------
# Override for the exploration phase, e.g.
#   terraform apply -var cpu_target=50
variable "min_capacity" {
  type    = number
  default = 2
}

variable "max_capacity" {
  type    = number
  default = 4
}

variable "cpu_target" {
  type    = number
  default = 70
}

variable "scale_out_cooldown" {
  type    = number
  default = 300
}

variable "scale_in_cooldown" {
  type    = number
  default = 300
}
