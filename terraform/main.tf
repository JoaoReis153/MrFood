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

module "cloudsql_foundation" {
  source = "./modules/cloudsql-foundation"

  project_id = var.project_id
  network_id = module.vpc.network_id

  depends_on = [module.vpc]
}



module "service_cloudsql" {
  for_each = var.service_databases
  source   = "./modules/cloudsql-postgres"

  project_id          = var.project_id
  region              = coalesce(each.value.region, var.region)
  instance_name       = "mrfood-${each.key}-pg"
  db_name             = each.value.db_name
  db_user             = each.value.db_user
  db_password         = each.value.db_password
  tier                = each.value.tier
  disk_size           = each.value.disk_size
  availability_type   = each.value.availability_type
  deletion_protection = each.value.deletion_protection
  private_network     = module.vpc.network_id

  depends_on = [module.cloudsql_foundation]
}


resource "google_project_service" "redis" {
  project = var.project_id
  service = "redis.googleapis.com"
}

module "service_redis" {
  for_each = var.service_redis_instances
  source   = "./modules/memorystore-redis"

  project_id         = var.project_id
  instance_name      = "mrfood-${each.key}-redis"
  region             = coalesce(each.value.region, var.region)
  location_id        = try(each.value.location_id, null)
  authorized_network = module.vpc.network_id

  tier                    = each.value.tier
  memory_size_gb          = each.value.memory_size_gb
  redis_version           = each.value.redis_version
  connect_mode            = each.value.connect_mode
  auth_enabled            = each.value.auth_enabled
  transit_encryption_mode = each.value.transit_encryption_mode
  labels                  = each.value.labels

  depends_on = [module.vpc, google_project_service.redis]
}


