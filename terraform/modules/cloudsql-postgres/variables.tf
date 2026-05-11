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
}

variable "db_name" {
  description = "PostgreSQL database name"
  type        = string
}

variable "db_user" {
  description = "PostgreSQL user name"
  type        = string
}

variable "db_password" {
  description = "PostgreSQL user password"
  type        = string
  sensitive   = true
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
  default     = true
}

variable "private_network" {
  description = "Self link of the VPC network for private IP"
  type        = string
}
