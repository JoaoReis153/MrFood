variable "project_id" {
  description = "GCP project ID"
  type        = string
  default     = "mrfood-490623"
}

variable "region" {
  description = "GCP region (Madrid)"
  type        = string
  default     = "europe-southwest1"
}

variable "network_name" {
  description = "Name of the VPC network"
  type        = string
  default     = "gke-network"
}

variable "subnet_name" {
  description = "Name of the subnet"
  type        = string
  default     = "gke-subnet"
}

variable "subnet_cidr" {
  description = "Primary CIDR range for the subnet"
  type        = string
  default     = "10.0.0.0/20"
}

variable "pods_cidr_name" {
  description = "Name of the secondary range for pods"
  type        = string
  default     = "pods"
}

variable "pods_cidr_range" {
  description = "CIDR range for pods"
  type        = string
  default     = "10.4.0.0/14"
}

variable "services_cidr_name" {
  description = "Name of the secondary range for services"
  type        = string
  default     = "services"
}

variable "services_cidr_range" {
  description = "CIDR range for services"
  type        = string
  default     = "10.0.32.0/20"
}

variable "cluster_name" {
  description = "Name of the GKE cluster"
  type        = string
  default     = "mrfood-cluster"
}

variable "node_machine_type" {
  description = "Machine type for the node pool"
  type        = string
  default     = "e2-standard-2"
}

variable "node_min_count" {
  description = "Minimum number of nodes"
  type        = number
  default     = 0
}

variable "node_max_count" {
  description = "Maximum number of nodes"
  type        = number
  default     = 5
}

variable "node_disk_size_gb" {
  description = "Disk size in GB for each node"
  type        = number
  default     = 20
}

variable "node_disk_type" {
  description = "Disk type for each node"
  type        = string
  default     = "pd-standard"
}

variable "node_preemptible" {
  description = "Whether nodes are preemptible"
  type        = bool
  default     = false
}

variable "repository_id" {
  description = "Name of the Artifact Registry repository"
  type        = string
  default     = "mrfood-repo"
}
