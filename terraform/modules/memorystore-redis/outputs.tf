output "host" {
  description = "Redis host IP"
  value       = google_redis_instance.this.host
}

output "port" {
  description = "Redis port"
  value       = google_redis_instance.this.port
}

output "current_location_id" {
  description = "Zone where Redis is running"
  value       = google_redis_instance.this.current_location_id
}

output "auth_string" {
  description = "Redis AUTH string (if enabled)"
  value       = google_redis_instance.this.auth_string
  sensitive   = true
}
