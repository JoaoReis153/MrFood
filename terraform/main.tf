terraform {
  backend "gcs" {
    bucket = "tf-state-manager-493721"
    prefix = "mrfood/prod"
  }
}

module "vpc" {
  source = "./modules/vpc"

  project_id   = var.project_id
  region       = var.region
  network_name = var.network_name
  subnet_name  = var.subnet_name

  subnet_cidr         = var.subnet_cidr
  pods_cidr_name      = var.pods_cidr_name
  pods_cidr_range     = var.pods_cidr_range
  services_cidr_name  = var.services_cidr_name
  services_cidr_range = var.services_cidr_range
}

module "gke" {
  source = "./modules/gke"

  project_id   = var.project_id
  region       = var.region
  cluster_name = var.cluster_name

  network    = module.vpc.network_name
  subnetwork = module.vpc.subnet_name

  pods_cidr_name     = var.pods_cidr_name
  services_cidr_name = var.services_cidr_name

  node_machine_type = var.node_machine_type
  node_min_count    = var.node_min_count
  node_max_count    = var.node_max_count
  node_disk_size_gb = var.node_disk_size_gb
  node_disk_type    = var.node_disk_type
  node_preemptible  = var.node_preemptible

  depends_on = [module.vpc]
}

module "registry" {
  source = "./modules/registry"

  project_id    = var.project_id
  region        = var.region
  repository_id = var.repository_id
}
