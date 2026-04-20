resource "google_container_cluster" "this" {
  name     = var.cluster_name
  location = var.region

  network    = var.network
  subnetwork = var.subnetwork

  remove_default_node_pool = true
  initial_node_count       = 1

  ip_allocation_policy {
    cluster_secondary_range_name  = var.pods_cidr_name
    services_secondary_range_name = var.services_cidr_name
  }
}

resource "google_container_node_pool" "primary" {
  name     = "${var.cluster_name}-node-pool"
  cluster  = google_container_cluster.this.name
  location = var.region

  node_count = var.node_count

  node_config {
    machine_type = var.node_machine_type

    oauth_scopes = [
      "https://www.googleapis.com/auth/cloud-platform"
    ]
  }
}

resource "google_container_node_pool" "primary" {
  name     = "${var.cluster_name}-node-pool"
  cluster  = google_container_cluster.this.name
  location = var.region

  node_count = var.node_count

  node_config {
    machine_type = var.node_machine_type

    oauth_scopes = [
      "https://www.googleapis.com/auth/cloud-platform"
    ]
  }
}
