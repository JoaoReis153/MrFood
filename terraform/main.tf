terraform {
  backend "gcs" {
    bucket = "tf-state-manager-493721"
    prefix = "mrfood/prod"
  }
}

# Grant Cloud SQL Admin role to terraform-sa service account for schema imports
resource "google_project_iam_member" "terraform_sa_cloudsql_admin" {
  project = var.project_id
  role    = "roles/cloudsql.admin"
  member  = "serviceAccount:terraform-sa@state-manager-493721.iam.gserviceaccount.com"
}

# Grant Storage Admin role to terraform-sa for bucket access
resource "google_project_iam_member" "terraform_sa_storage_admin" {
  project = var.project_id
  role    = "roles/storage.admin"
  member  = "serviceAccount:terraform-sa@state-manager-493721.iam.gserviceaccount.com"
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

resource "google_project_service_identity" "cloudsql" {
  provider = google-beta
  project  = var.project_id
  service  = "sqladmin.googleapis.com"

  depends_on = [module.cloudsql_foundation]
}


data "google_project" "current" {
  project_id = var.project_id
}

locals {
  service_schema = {
    for svc, cfg in var.service_databases :
    svc => {
      bootstrap_enabled = try(cfg.bootstrap_enabled, true)
      schema_revision   = try(cfg.schema_revision, "v1")
      schema_sql_path   = coalesce(try(cfg.schema_sql_path, null), "${path.root}/../services/${svc}/db_setup.sql")
    }
  }
}

resource "google_storage_bucket" "schema_bootstrap" {
  name                        = var.schema_bootstrap_bucket_name
  project                     = var.project_id
  location                    = var.region
  uniform_bucket_level_access = true
}

resource "google_project_iam_member" "cloudsql_admin_cloudsql" {
  project = var.project_id
  role    = "roles/cloudsql.admin"
  member  = google_project_service_identity.cloudsql.member

  depends_on = [google_project_service_identity.cloudsql]
}

resource "google_project_iam_member" "cloudsql_storage_admin" {
  project = var.project_id
  role    = "roles/storage.admin"
  member  = google_project_service_identity.cloudsql.member

  depends_on = [google_project_service_identity.cloudsql]
}

resource "google_storage_bucket_iam_member" "cloudsql_schema_reader" {
  bucket = google_storage_bucket.schema_bootstrap.name
  role   = "roles/storage.objectViewer"
  member = google_project_service_identity.cloudsql.member

  depends_on = [time_sleep.wait_for_cloudsql_service_identity]
}

resource "google_storage_bucket_iam_member" "cloudsql_schema_bucket_reader" {
  bucket = google_storage_bucket.schema_bootstrap.name
  role   = "roles/storage.admin"
  member = google_project_service_identity.cloudsql.member

  depends_on = [time_sleep.wait_for_cloudsql_service_identity]
}


resource "google_storage_bucket_object" "service_schema_sql" {
  for_each = {
    for svc, cfg in local.service_schema :
    svc => cfg
    if cfg.bootstrap_enabled && fileexists(cfg.schema_sql_path)
  }

  bucket = google_storage_bucket.schema_bootstrap.name
  name   = "schemas/${each.key}/${each.value.schema_revision}-${filesha256(each.value.schema_sql_path)}.sql"
  source = each.value.schema_sql_path
}

resource "time_sleep" "wait_for_iam_propagation" {
  create_duration = "60s"
  depends_on = [
    google_project_iam_member.terraform_sa_cloudsql_admin,
    google_project_iam_member.terraform_sa_storage_admin,
    google_project_iam_member.cloudsql_storage_admin,
    google_storage_bucket_iam_member.cloudsql_schema_reader,
    google_storage_bucket_iam_member.cloudsql_schema_bucket_reader
  ]
}

resource "terraform_data" "apply_service_schema" {
  for_each = google_storage_bucket_object.service_schema_sql

  triggers_replace = [
    module.service_cloudsql[each.key].instance_name,
    module.service_cloudsql[each.key].private_ip_address,
    local.service_schema[each.key].schema_revision,
    google_storage_bucket_object.service_schema_sql[each.key].name,
  ]

  provisioner "local-exec" {
    interpreter = ["/bin/bash", "-c"]
    command     = <<-EOT
      set -euo pipefail
      gcloud sql import sql "${module.service_cloudsql[each.key].instance_name}" \
        "gs://${google_storage_bucket.schema_bootstrap.name}/${google_storage_bucket_object.service_schema_sql[each.key].name}" \
        --database="${var.service_databases[each.key].db_name}" \
        --project="${var.project_id}" \
        --quiet
    EOT
  }

  depends_on = [
    module.service_cloudsql,
    google_project_iam_member.terraform_sa_cloudsql_admin,
    google_project_iam_member.terraform_sa_storage_admin,
    google_storage_bucket_iam_member.cloudsql_schema_reader,
    google_storage_bucket_iam_member.cloudsql_schema_bucket_reader,
    time_sleep.wait_for_iam_propagation
  ]
}

resource "time_sleep" "wait_for_cloudsql_service_identity" {
  create_duration = "90s"
  depends_on      = [google_project_service_identity.cloudsql]
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


