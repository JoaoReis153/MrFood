output "network_name" {
  description = "The name of the VPC network"
  value       = google_compute_network.vpc_network.name
}

output "network_self_link" {
  description = "The self link of the VPC network"
  value       = google_compute_network.vpc_network.self_link
}

output "subnetwork_name" {
  description = "The name of the VPC subnetwork"
  value       = google_compute_subnetwork.gke.name
}

output "subnetwork_self_link" {
  description = "The self link of the VPC subnetwork"
  value       = google_compute_subnetwork.gke.self_link
}
