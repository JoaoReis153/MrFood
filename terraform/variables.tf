variable "project_id" {
  type = string
}

variable "region" {
  type = string
}

variable "zone" {
  type = string
}

variable "network_name" {
  type = string
}

variable "cluster_name" {
  type        = string
  description = "Name of the GKE cluster"
}

variable "node_machine_type" {
  type = string
}

variable "node_count" {
  type = number
}
