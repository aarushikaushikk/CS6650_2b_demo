variable "service_name" {
  type        = string
  description = "Base name for ALB resources"
}

variable "vpc_id" {
  type        = string
  description = "VPC for the ALB and target group"
}

variable "subnet_ids" {
  type        = list(string)
  description = "Subnets (at least 2 AZs) for the ALB"
}

variable "container_port" {
  type        = number
  description = "Port the tasks listen on"
}

variable "deregistration_delay" {
  type        = number
  default     = 30
  description = "Seconds the ALB keeps draining a target before removing it"
}
