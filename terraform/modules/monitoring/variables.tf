variable "service_name" {
  type = string
}

variable "cluster_name" {
  type = string
}

variable "region" {
  type = string
}

variable "alb_arn_suffix" {
  type = string
}

variable "target_group_arn_suffix" {
  type = string
}

variable "cpu_target" {
  type        = number
  description = "Drawn as a reference line on the CPU widget"
}
