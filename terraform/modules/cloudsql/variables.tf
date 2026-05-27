variable "project_id" {
  description = "GCP project ID"
  type        = string
}

variable "region" {
  description = "Region for the Cloud SQL instance"
  type        = string
}

variable "instance_name" {
  description = "Cloud SQL instance name"
  type        = string
  default     = "mrfood-pg"
}

variable "tier" {
  description = "Cloud SQL machine tier"
  type        = string
  default     = "db-f1-micro"
}

variable "disk_size" {
  description = "Disk size in GB"
  type        = number
  default     = 20
}

variable "availability_type" {
  description = "ZONAL or REGIONAL"
  type        = string
  default     = "ZONAL"
}

variable "deletion_protection" {
  description = "Protect instance from accidental deletion"
  type        = bool
  default     = false
}

variable "private_network" {
  description = "Self link of the VPC network for private IP"
  type        = string
}

variable "databases" {
  description = "Map of service name to database credentials"
  type = map(object({
    db_name     = string
    db_user     = string
    db_password = string
  }))
}
