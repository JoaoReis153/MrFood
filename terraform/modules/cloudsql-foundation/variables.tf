variable "project_id" {
  description = "GCP project ID"
  type        = string
}

variable "network_id" {
  description = "VPC network self link/id"
  type        = string
}

variable "private_range_name" {
  description = "Name of the private service networking IP range"
  type        = string
  default     = "cloudsql-private-ip-range"
}

variable "private_range_prefix_length" {
  description = "CIDR prefix length for private service networking range"
  type        = number
  default     = 16
}
