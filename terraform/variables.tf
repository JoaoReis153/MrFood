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

variable "service_databases" {
  description = "Per-service Cloud SQL Postgres configuration"
  type = map(object({
    db_name             = string
    db_user             = string
    db_password         = string
    region              = optional(string, "europe-southwest1")
    tier                = optional(string, "db-f1-micro")
    disk_size           = optional(number, 20)
    availability_type   = optional(string, "ZONAL")
    deletion_protection = optional(bool, true)

    bootstrap_enabled = optional(bool, true)
    schema_sql_path   = optional(string)
    schema_revision   = optional(string, "v1")
  }))

  default = {
    auth = {
      db_name     = "mrfood_auth"
      db_user     = "mrfood_auth_user"
      db_password = "mrfood_auth_secret"
    }
    restaurant = {
      db_name     = "mrfood_restaurant"
      db_user     = "mrfood_restaurant_user"
      db_password = "mrfood_restaurant_secret"
    }
    booking = {
      db_name     = "mrfood_booking"
      db_user     = "mrfood_booking_user"
      db_password = "mrfood_booking_secret"
    }
    review = {
      db_name     = "mrfood_review"
      db_user     = "mrfood_review_user"
      db_password = "mrfood_review_secret"
    }
    payment = {
      db_name     = "mrfood_payment"
      db_user     = "mrfood_payment_user"
      db_password = "mrfood_payment_secret"
    }
    sponsor = {
      db_name     = "mrfood_sponsor"
      db_user     = "mrfood_sponsor_user"
      db_password = "mrfood_sponsor_secret"
    }
  }
}


variable "service_redis_instances" {
  description = "Per-service Memorystore Redis configuration"
  type = map(object({
    region                  = optional(string, "europe-southwest1")
    location_id             = optional(string)
    tier                    = optional(string, "BASIC")
    memory_size_gb          = optional(number, 1)
    redis_version           = optional(string, "REDIS_7_2")
    connect_mode            = optional(string, "DIRECT_PEERING")
    auth_enabled            = optional(bool, false)
    transit_encryption_mode = optional(string, "DISABLED")
    labels                  = optional(map(string), {})
  }))

  default = {
    auth = {
      memory_size_gb = 1
      labels = {
        service = "auth"
      }
    }
    notification = {
      memory_size_gb = 1
      labels = {
        service = "notification"
      }
    }
  }
}

variable "schema_bootstrap_bucket_name" {
  description = "GCS bucket used to stage SQL bootstrap files for Cloud SQL import"
  type        = string
  default     = "mrfood-cloudsql-schema-bootstrap-490623"
}
