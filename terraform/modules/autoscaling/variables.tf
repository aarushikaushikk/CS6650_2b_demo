variable "cluster_name" {
  type = string
}

variable "service_name" {
  type = string
}

variable "min_capacity" {
  type = number
}

variable "max_capacity" {
  type = number
}

variable "cpu_target" {
  type        = number
  description = "Target average CPU % for target tracking"
}

variable "scale_out_cooldown" {
  type = number
}

variable "scale_in_cooldown" {
  type = number
}
