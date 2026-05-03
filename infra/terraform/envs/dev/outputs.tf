output "gke_cluster_name" { value = module.gke.cluster_name }
output "gke_location" { value = module.gke.location }
output "postgres_connection_name" { value = module.postgres.connection_name }
output "redis_host" { value = module.redis.host }
output "artifact_registry_repository" { value = module.artifact_registry.repository_id }
