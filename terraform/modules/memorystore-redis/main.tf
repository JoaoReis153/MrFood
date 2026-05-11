resource "google_redis_instance" "this" {
  project            = var.project_id
  name               = var.instance_name
  tier               = var.tier
  memory_size_gb     = var.memory_size_gb
  region             = var.region
  location_id        = var.location_id
  redis_version      = var.redis_version
  connect_mode       = var.connect_mode
  authorized_network = var.authorized_network

  auth_enabled            = var.auth_enabled
  transit_encryption_mode = var.transit_encryption_mode
  read_replicas_mode      = var.read_replicas_mode
  replica_count           = var.replica_count

  labels = var.labels
}
