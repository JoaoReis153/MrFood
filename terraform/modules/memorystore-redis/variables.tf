variable "project_id" {
  description = "GCP project ID"
  type        = string
}

variable "instance_name" {
  description = "Redis instance name"
  type        = string
}

variable "region" {
  description = "Redis region"
  type        = string
}

variable "location_id" {
  description = "Optional zone within the region"
  type        = string
  default     = null
}

variable "authorized_network" {
  description = "VPC network self link for private service access"
  type        = string
}

variable "tier" {
  description = "Redis tier (BASIC or STANDARD_HA)"
  type        = string
  default     = "BASIC"
}

variable "memory_size_gb" {
  description = "Redis memory in GB"
  type        = number
  default     = 1
}

variable "redis_version" {
  description = "Redis version"
  type        = string
  default     = "REDIS_7_2"
}

variable "connect_mode" {
  description = "Connection mode"
  type        = string
  default     = "DIRECT_PEERING"
}

variable "auth_enabled" {
  description = "Enable Redis AUTH"
  type        = bool
  default     = false
}

variable "transit_encryption_mode" {
  description = "TLS mode (DISABLED or SERVER_AUTHENTICATION)"
  type        = string
  default     = "DISABLED"
}

variable "read_replicas_mode" {
  description = "Read replicas mode (optional)"
  type        = string
  default     = "READ_REPLICAS_DISABLED"
}

variable "replica_count" {
  description = "Replica count for STANDARD_HA/read replicas"
  type        = number
  default     = 0
}

variable "labels" {
  description = "Resource labels"
  type        = map(string)
  default     = {}
}
