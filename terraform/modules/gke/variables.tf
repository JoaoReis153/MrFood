variable "project_id" {
  description = "GCP project ID"
  type        = string
}

variable "region" {
  description = "GCP region"
  type        = string
}

variable "cluster_name" {
  description = "Name of the GKE cluster"
  type        = string
}

variable "network" {
  description = "VPC network name"
  type        = string
}

variable "subnetwork" {
  description = "Subnetwork name"
  type        = string
}

variable "pods_cidr_name" {
  description = "Name of the secondary range for pods"
  type        = string
}

variable "services_cidr_name" {
  description = "Name of the secondary range for services"
  type        = string
}

variable "master_ipv4_cidr_block" {
  description = "CIDR block for the master nodes (must not overlap with other ranges)"
  type        = string
  default     = "172.16.0.0/28"
}

variable "master_authorized_networks" {
  description = "List of CIDR blocks allowed to access the Kubernetes master"
  type = list(object({
    cidr_block   = string
    display_name = string
  }))
  default = [
    {
      cidr_block   = "0.0.0.0/0"
      display_name = "all"
    }
  ]
}

variable "release_channel" {
  description = "GKE release channel (RAPID, REGULAR, STABLE)"
  type        = string
  default     = "REGULAR"

  validation {
    condition     = contains(["RAPID", "REGULAR", "STABLE"], var.release_channel)
    error_message = "release_channel must be one of: RAPID, REGULAR, STABLE."
  }
}

variable "node_machine_type" {
  description = "Machine type for the node pool"
  type        = string
}

variable "node_min_count" {
  description = "Minimum number of nodes"
  type        = number
}

variable "node_max_count" {
  description = "Maximum number of nodes"
  type        = number
}

variable "node_disk_size_gb" {
  description = "Disk size in GB for each node"
  type        = number
}

variable "node_disk_type" {
  description = "Disk type for each node"
  type        = string
}

variable "node_preemptible" {
  description = "Whether nodes are preemptible"
  type        = bool
}
