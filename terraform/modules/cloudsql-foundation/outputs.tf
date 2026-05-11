output "private_range_name" {
  description = "Allocated private range name"
  value       = google_compute_global_address.private_ip_alloc.name
}

output "private_vpc_connection_id" {
  description = "Service networking connection id"
  value       = google_service_networking_connection.private_vpc_connection.id
}
