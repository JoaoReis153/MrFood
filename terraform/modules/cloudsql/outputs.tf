output "instance_name" {
  description = "Cloud SQL instance name"
  value       = google_sql_database_instance.this.name
}

output "connection_name" {
  description = "Instance connection name (<project>:<region>:<instance>)"
  value       = google_sql_database_instance.this.connection_name
}

output "private_ip_address" {
  description = "Private IP address of the Cloud SQL instance"
  value       = google_sql_database_instance.this.private_ip_address
}

output "service_account_email" {
  description = "Service account email used by the Cloud SQL instance"
  value       = google_sql_database_instance.this.service_account_email_address
}
