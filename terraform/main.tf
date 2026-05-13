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
  zone         = var.cluster_zone
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

resource "terraform_data" "force_delete_vpc_peering" {
  input = {
    network = module.vpc.network_name
    project = var.project_id
  }

  provisioner "local-exec" {
    when    = destroy
    command = <<-EOT
      gcloud services vpc-peerings delete \
        --service=servicenetworking.googleapis.com \
        --network=${self.input.network} \
        --project=${self.input.project} \
        --force --quiet || true
    EOT
  }
}

module "cloudsql" {
  source = "./modules/cloudsql"

  project_id      = var.project_id
  region          = var.region
  instance_name   = var.cloudsql_instance_name
  tier            = var.cloudsql_tier
  disk_size       = var.cloudsql_disk_size
  private_network = module.vpc.network_id
  databases       = var.service_databases

  depends_on = [module.cloudsql_foundation, terraform_data.force_delete_vpc_peering]
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
      schema_revision = "v1"
      schema_sql_path = "${path.root}/../services/${svc}/db_setup.sql"
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

resource "time_sleep" "wait_for_cloudsql_service_identity" {
  create_duration = "30s"
  depends_on      = [google_project_service_identity.cloudsql]
}

resource "google_storage_bucket_iam_member" "cloudsql_schema_reader" {
  bucket = google_storage_bucket.schema_bootstrap.name
  role   = "roles/storage.objectViewer"
  member = google_project_service_identity.cloudsql.member

  depends_on = [time_sleep.wait_for_cloudsql_service_identity]
}

resource "google_storage_bucket_iam_member" "cloudsql_schema_bucket_reader" {
  bucket = google_storage_bucket.schema_bootstrap.name
  role   = "roles/storage.legacyBucketReader"
  member = google_project_service_identity.cloudsql.member

  depends_on = [time_sleep.wait_for_cloudsql_service_identity]
}

resource "google_storage_bucket_iam_member" "instance_schema_reader" {
  bucket = google_storage_bucket.schema_bootstrap.name
  role   = "roles/storage.objectViewer"
  member = "serviceAccount:${module.cloudsql.service_account_email}"
}

resource "google_storage_bucket_object" "service_schema_sql" {
  for_each = {
    for svc, cfg in local.service_schema :
    svc => cfg
    if fileexists(cfg.schema_sql_path)
  }

  bucket = google_storage_bucket.schema_bootstrap.name
  name   = "schemas/${each.key}/${each.value.schema_revision}-${filesha256(each.value.schema_sql_path)}.sql"
  source = each.value.schema_sql_path
}

resource "time_sleep" "wait_for_iam_propagation" {
  create_duration = "30s"
  depends_on = [
    google_project_iam_member.terraform_sa_cloudsql_admin,
    google_project_iam_member.terraform_sa_storage_admin,
    google_storage_bucket_iam_member.cloudsql_schema_reader,
    google_storage_bucket_iam_member.cloudsql_schema_bucket_reader,
    google_storage_bucket_iam_member.instance_schema_reader,
  ]
}

resource "terraform_data" "apply_service_schema" {
  triggers_replace = [
    module.cloudsql.instance_name,
    module.cloudsql.private_ip_address,
    jsonencode({ for svc, obj in google_storage_bucket_object.service_schema_sql : svc => obj.name }),
  ]

  provisioner "local-exec" {
    interpreter = ["/bin/bash", "-c"]
    command     = <<-EOT
      set -euo pipefail
      %{~for svc, obj in google_storage_bucket_object.service_schema_sql}
      echo "Importing schema for ${svc}..."
      gcloud sql import sql "${module.cloudsql.instance_name}" \
        "gs://${google_storage_bucket.schema_bootstrap.name}/${obj.name}" \
        --database="${var.service_databases[svc].db_name}" \
        --project="${var.project_id}" \
        --quiet
      %{~endfor}
    EOT
  }

  depends_on = [
    module.cloudsql,
    google_project_iam_member.terraform_sa_cloudsql_admin,
    google_project_iam_member.terraform_sa_storage_admin,
    google_storage_bucket_iam_member.cloudsql_schema_reader,
    google_storage_bucket_iam_member.cloudsql_schema_bucket_reader,
    time_sleep.wait_for_iam_propagation,
  ]
}

# ──────────────────────────────────────────────────────────────────────────────
# Workload Identity — one GCP service account per k8s service
# ──────────────────────────────────────────────────────────────────────────────

locals {
  cloudsql_services = toset(["auth", "restaurant", "booking", "review", "payment", "sponsor", "cdc"])
  all_services      = toset(["auth", "restaurant", "booking", "review", "payment", "sponsor", "notification", "search", "otel-collector", "cdc"])
}

resource "google_service_account" "service_sa" {
  for_each = local.all_services

  account_id   = "${each.key}-sa"
  display_name = "Workload Identity SA for ${each.key}"
  project      = var.project_id
}

resource "google_service_account_iam_member" "workload_identity" {
  for_each = local.all_services

  service_account_id = google_service_account.service_sa[each.key].name
  role               = "roles/iam.workloadIdentityUser"
  member             = "serviceAccount:${var.project_id}.svc.id.goog[mrfood/${each.key}]"
}

resource "google_project_iam_member" "service_cloudsql_client" {
  for_each = local.cloudsql_services

  project = var.project_id
  role    = "roles/cloudsql.client"
  member  = "serviceAccount:${google_service_account.service_sa[each.key].email}"
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

  depends_on = [module.vpc, google_project_service.redis, module.cloudsql_foundation, terraform_data.force_delete_vpc_peering]
}
