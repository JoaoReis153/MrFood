terraform {
  required_version = ">= 1.5.0"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "7.28.0"
    }
    google-beta = {
      source  = "hashicorp/google-beta"
      version = "7.28.0"
    }
    time = {
      source  = "hashicorp/time"
      version = "~> 0.12"
    }

  }
}

provider "google" {
  project = var.project_id
  region  = var.region
}

provider "google-beta" {
  project = var.project_id
  region  = var.region
}