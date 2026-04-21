terraform {
  required_version = ">= 1.5.0"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "7.28.0"
    }
  }

  # Uncomment and configure for remote state
  # backend "gcs" {
  #   bucket = "your-terraform-state-bucket"
  #   prefix = "terraform/gke-madrid"
  # }
}

provider "google" {
  project = var.project_id
  region  = var.region
}
