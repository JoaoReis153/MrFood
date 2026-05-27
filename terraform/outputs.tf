output "vpc_network_name" {
  description = "Name of the VPC network"
  value       = module.vpc.network_name
}

output "vpc_subnet_name" {
  description = "Name of the subnet"
  value       = module.vpc.subnet_name
}

output "gke_cluster_name" {
  description = "Name of the GKE cluster"
  value       = module.gke.cluster_name
}

output "gke_cluster_endpoint" {
  description = "Endpoint of the GKE cluster"
  value       = module.gke.cluster_endpoint
  sensitive   = true
}

output "gke_cluster_ca_certificate" {
  description = "CA certificate of the GKE cluster"
  value       = module.gke.cluster_ca_certificate
  sensitive   = true
}

output "db_private_ip" {
  description = "Private IP of the shared Cloud SQL instance"
  value       = module.cloudsql.private_ip_address
}

output "db_connection_name" {
  description = "Connection name of the shared Cloud SQL instance"
  value       = module.cloudsql.connection_name
}

output "service_redis_hosts" {
  description = "Redis host IP per service"
  value = {
    for svc, m in module.service_redis : svc => m.host
  }
}

output "service_redis_ports" {
  description = "Redis port per service"
  value = {
    for svc, m in module.service_redis : svc => m.port
  }
}

output "service_redis_auth_strings" {
  description = "Redis AUTH string per service (if enabled)"
  value = {
    for svc, m in module.service_redis : svc => m.auth_string
  }
  sensitive = true
}
