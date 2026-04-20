terraform {
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "7.0.0"
    }
  }
}

provider "google" {
  project = var.project_id
  region  = var.region
  zone    = var.zone
}

module "vpc" {
  source       = "./modules/vpc"
  network_name = var.network_name
  region       = var.region
}

module "gke" {
  source             = "./modules/gke"
  subnetwork         = module.vpc.subnetwork_name
  network            = module.vpc.network_name
  cluster_name       = "mrfood-cluster"
  node_machine_type  = "e2-standard-2"
  node_count         = 1
  region             = var.region
  pods_cidr_name     = "pods"
  services_cidr_name = "services"
}
