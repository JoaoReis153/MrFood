variable "project_id" {
  description = "GCP project ID"
  type        = string
}

variable "region" {
  description = "GCP region"
  type        = string
}

variable "network_name" {
  description = "Name of the VPC network"
  type        = string
}

variable "subnet_name" {
  description = "Name of the subnet"
  type        = string
}

variable "subnet_cidr" {
  description = "Primary CIDR range for the subnet"
  type        = string
}

variable "pods_cidr_name" {
  description = "Name of the secondary range for pods"
  type        = string
}

variable "pods_cidr_range" {
  description = "CIDR range for pods"
  type        = string
}

variable "services_cidr_name" {
  description = "Name of the secondary range for services"
  type        = string
}

variable "services_cidr_range" {
  description = "CIDR range for services"
  type        = string
}
