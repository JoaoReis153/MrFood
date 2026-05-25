resource "google_sql_database_instance" "this" {
  name                = var.instance_name
  project             = var.project_id
  region              = var.region
  database_version    = "POSTGRES_15"
  deletion_protection = var.deletion_protection

  settings {
    tier              = var.tier
    disk_size         = var.disk_size
    disk_type         = "PD_SSD"
    availability_type = var.availability_type

    ip_configuration {
      ipv4_enabled    = false
      private_network = var.private_network
    }

    backup_configuration {
      enabled = false
    }

    database_flags {
      name  = "cloudsql.logical_decoding"
      value = "on"
    }
  }
}

resource "google_sql_database" "db" {
  for_each = var.databases

  name     = each.value.db_name
  project  = var.project_id
  instance = google_sql_database_instance.this.name

  # ABANDON skips the Cloud SQL API delete call and lets the instance
  # deletion wipe the database. This avoids errors from active connections
  # (cloud-sql-proxy) or logical replication slots left by Debezium CDC.
  deletion_policy = "ABANDON"
}

resource "google_sql_user" "user" {
  for_each = var.databases

  name     = each.value.db_user
  project  = var.project_id
  instance = google_sql_database_instance.this.name
  password = each.value.db_password

  # Same reason: user deletion fails when the DB still has CDC-owned objects.
  deletion_policy = "ABANDON"
}
