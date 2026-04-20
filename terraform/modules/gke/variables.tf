variable "cluster_name" {
  type        = string
  description = "Name of the GKE cluster"
}

variable "region" {
  type = string
}

variable "network" {
  type        = string
  description = "VPC network self_link or name"
}

variable "subnetwork" {
  type        = string
  description = "Subnet name or self_link"
}

variable "pods_cidr_name" {
  type        = string
  description = "Secondary range name for pods"
}

variable "services_cidr_name" {
  type        = string
  description = "Secondary range name for services"
}

variable "node_machine_type" {
  type = string
}

variable "node_min" {
  type = number
}

variable "node_max" {
  type = number
}
