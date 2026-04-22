output "private_ip_address" {
  description = "Private IP address of the Cloud SQL instance"
  value       = google_sql_database_instance.this.private_ip_address
}

output "connection_name" {
  description = "Instance connection name (<project>:<region>:<instance>)"
  value       = google_sql_database_instance.this.connection_name
}

output "db_name" {
  description = "Created database name"
  value       = google_sql_database.db.name
}

output "db_user" {
  description = "Created database user"
  value       = google_sql_user.user.name
}

output "instance_name" {
  description = "Cloud SQL instance name"
  value       = google_sql_database_instance.this.name
}
