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
  cluster_name       = var.cluster_name
  node_machine_type  = var.node_machine_type
  node_min           = var.node_min
  node_max           = var.node_max
  region             = var.region
  pods_cidr_name     = "pods"
  services_cidr_name = "services"
}
