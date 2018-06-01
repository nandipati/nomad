data_dir = "/opt/nomad/data"
bind_addr = "0.0.0.0"

# Enable the server
server {
  enabled = true
  bootstrap_expect = SERVER_COUNT
  server_join {
    retry_join = ["RETRY_JOIN"]
    retry_max = 3
    retry_interval = "15s"
  }
}



vault {
  enabled = false
  address = "vault.service.consul"
  task_token_ttl = "1h"
  create_from_role = "nomad-cluster"
  token = ""
}

