#!/bin/bash

set -euo pipefail

unset CDPATH

cd "$(dirname "$0")"

# rm -rf data
mkdir -p data

cat > server.hcl <<EOF
data_dir                    = "./data"
server                      = true
bind_addr                   = "127.0.0.1"
client_addr                 = "127.0.0.1"
advertise_addr              = "127.0.0.1"
bootstrap_expect            = 1
disable_anonymous_signature = true
# log_level = "DEBUG"
connect {
  enabled = true
}
ports {
  grpc = 8502
}
ui_config {
  enabled = true
}
performance = {
  raft_multiplier = 1
}

limits {
  rpc_max_conns_per_client  = 10000
  http_max_conns_per_client = 10000
}
EOF


exec consul agent -config-file=server.hcl
