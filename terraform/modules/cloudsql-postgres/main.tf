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
      enabled = true
    }
  }
}

resource "google_sql_database" "db" {
  name     = var.db_name
  project  = var.project_id
  instance = google_sql_database_instance.this.name
}

resource "google_sql_user" "user" {
  name     = var.db_user
  project  = var.project_id
  instance = google_sql_database_instance.this.name
  password = var.db_password
}
