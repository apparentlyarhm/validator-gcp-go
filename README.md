# Bastion

## Introduction

This repository contains the backend service for <i>Bastion</i>, a Custom Minecraft Control Panel. 
Built with Golang, this application provides a REST API to interface with a 
Minecraft server hosted on Google Cloud Platform.

The application was originally written in Spring boot [here](https://github.com/apparentlyarhm/validator-gcp-java) but was ported for
instant cold start times on cloud run and generally less docker image size, trade off with DX

The service is designed to be hosted on Google Cloud Run. It handles everything from user 
authentication and GCP infrastructure management to direct communication with the Minecraft server itself.

It is vital that firewall rules for RCON and QUERY are updated accordingly to allow traffic for it. Also keep
the RCON password strong.

## Core Features

- Google Cloud Platform Integration: Natively interacts with GCP services using the official client libraries:
    - **Compute Engine**: To fetch details and manage the state of the Minecraft server's Virtual Machine
    - **Firewall**: enabling IP whitelisting for players, as well as privatising or publicising the server
    - **GCS**: To read and generate download links for current mods. This bucket is populated using a script
        in the vm.

- Direct connection to the VM via TCP/UDP for:
  - **Query Protocol**: To fetch the Message of the Day (MOTD) and server status, general query of the state.
  - **RCON**: To securely execute commands on the server as an administrator.
  - **Logs**: Read log lines from `latest.log` via SSH.
    
  of course, these must be enabled in the `server.properties`

- Authentication & Authorization:
  - **GitHub OAuth2**: Handles user login and identity verification.
  - **JWTs**: Issued to authenticated users for securing API endpoints.

- Configuration
    
    Apart from standard ENV VARS, you can also whitelist GitHub IDs of normal users and Admins. that will directly 
    affect who is allowed to do what in case of non-public apis.
- Misc
    - Docker for deployment
    - `sh` scripts for CI/CD.

## API Overview

The API provides a clear and logical set of endpoints for managing the server.

### Authentication (`/api/v2/auth`)

- GET /login: Initiates the GitHub OAuth flow.
- GET /callback?code=val: The callback URI for GitHub to complete the login process and issue a JWT.

### Main Controller (`/api/v2`)

* GET /ping: A simple health-check endpoint.
* GET /machine: Retrieves details of the associated GCP Compute Engine VM.
* GET /firewall: Fetches the current firewall state, currently not in use anywhere
* GET /firewall/check-ip?ip=val: Checks if a specific IP address is currently whitelisted.
* GET /server-info?address=val: Gets the server's Message of the Day (MOTD), version, and player count.
* GET /metrics?address=val: Queries Prometheus for the current snapshot of server metrics, such as TPS, MSPT, players online, chunk counts, JVM memory, and CPU usage.
* GET /metrics/series?address=val&metric=tps&start=2026-01-01T00:00:00Z&end=2026-01-01T01:00:00Z: Returns a time-series array for a metric between two RFC3339 timestamps. The default step is 15s and the default range is the last hour when start/end are omitted.
* GET /mods [USER/ADMIN]: Lists all mods currently available on the server.
* GET /mods/download/{fileName} [USER/ADMIN]: Provides a download link or stream for a specific mod file.
* GET /logs?address=val&c=count: [USER/ADMIN] Gets a formatted list of logs with fields like Timestamp, level, source and the message it self.
* PATCH /firewall/add-ip: Adds the requesting user's IP address to the firewall whitelist.
* PATCH /firewall/purge: [ADMIN] Removes all IP addresses from the firewall whitelist, essentially preventing all public access.
* PATCH /firewall/make-public: [ADMIN] Opens the server to the public by setting the firewall rule to allow 0.0.0.0/0.
* GET /metrics?address=val: Queries Prometheus for the current snapshot of server metrics, such as TPS, MSPT, players online, chunk counts, JVM memory, and CPU usage.
* GET /metrics/series?address=val&metric=tps&start=2026-01-01T00:00:00Z&end=2026-01-01T01:00:00Z: Returns a time-series array for a metric between two RFC3339 timestamps. The default step is 15s and the default range is the last hour when start/end are omitted.

⚠️ **Warning**
This endpoint executes commands via RCON on your server.
Only whitelisted commands are allowed, and all executions are logged.
You can ovveride the list by using "Custom".
* POST /execute?address=val: [USER/ADMIN] Executes a command on the Minecraft server via RCON.

## Authentication?
All apis which are tagged with `ADMIN` or `USER/ADMIN` are NOT public. which means, the `Authorization` header must be supplied in the standard format: `Bearer <token>` where `<token>` is the server-issued JWT.

If you are using this app, make sure to supply it with your own client ID and secrets from github.

There are 3 roles that the server allocates. see `config.go`.
`USER`,`ADMIN` from the hardcoded slice defined, and `ANON` if it isnt in either list.

**Practically, the amount of priviledge `ANON` has is equal to not logging in at all.**

## Example Flow: Viewing Server Logs

1. Authenticate via GitHub OAuth
2. Receive JWT
3. Call `/logs?address=...&c=100` with `Authorization` header

## Environment
see `env.example`
```bash
# Oauth
GITHUB_CLIENT_ID=value
GITHUB_CLIENT_SECRET=value
FE_HOST=value

# for querying infra
GOOGLE_CLOUD_BUCKET_NAME=value
GOOGLE_CLOUD_FIREWALL_NAME=value
GOOGLE_CLOUD_PROJECT=value
GOOGLE_CLOUD_VM_NAME=value
GOOGLE_CLOUD_VM_ZONE=value

# minecraft ops - needs everything enabled
MINECRAFT_SERVER_PORT=value
MINECRAFT_QUERY_PORT=value # udp query port used by /serverinfo
MINECRAFT_RCON_PASS=value
MINECRAFT_RCON_PORT=value
SSH_LOG_PATH=path/to/latest.log

# validating jwts
SIGNING_SECRET=value

# to deploy
# A NOTE ON REGION:
# the reason its named "VM region" is because we usually keep the vm and rest of the things in the
# same region. so this same var can be reused. its sort of misleading but too lazy to change
GOOGLE_CLOUD_VM_REGION=value
GOOGLE_CLOUD_AR_REPO_NAME=value
GOOGLE_CLOUD_CR_SERVICE_NAME=value

# for permissions and identity
GOOGLE_APPLICATION_CREDENTIALS=value # for local
GOOGLE_SERVICE_ACCOUNT_EMAIL=value # for local, will be inferred in the deployment script

# for log fetching via ssh
SSH_PRIVATE_KEY_BASE64=value # get the ssh key and convert it into base64
SSH_VM_USER=value
SSH_HOST_KEY_HASH=value # it took me this long to add this idk why

# for prometheus-backed metric queries
PROMETHEUS_API_KEY=value # shared key expected by the exporter or reverse proxy
PROMETHEUS_PORT=value # usually 9090 or whichever port exposes the Prometheus API
PROMETHEUS_QUERY_PROFILE=default # switch to a named query set without deleting old ones
```

## Prometheus-backed metrics

The app can query a Prometheus-compatible exporter on the Minecraft VM and expose the result through the public API. The implementation expects:

- a Prometheus-compatible metrics endpoint reachable at `http://<vm-ip>:<PROMETHEUS_PORT>/api/v1/query`
- a shared API key in `PROMETHEUS_API_KEY` for authenticated access
- a valid VM IP passed to the `address` query parameter on the API routes

### Query profiles (recommended for MC version/loader changes)

Prometheus query names and labels can change when you change loader/modpacks or Minecraft versions. Instead of overwriting old expressions, I decided to keep them in named profiles and switch profiles via env:

- `PROMETHEUS_QUERY_PROFILE=some-val` (active profile)

Profile mappings are defined in `internal/models/intermediaries.go` and selected at runtime. This lets you:

- keep older query sets as reference
- safely add a new profile for a new server stack
- roll back quickly by changing only one env variable

### Available endpoints

- `GET /api/v2/metrics?address=<vm-ip>`
  - Returns a single snapshot of the current values for the configured metrics.
  - Supported metric keys include: `tps`, `mspt`, `players`, `entities`, `chunks`, `totalChunks`, `handshakes`, `jvmMem`, `jvmMemHeap`, `jvmMemMax`, `jvmMemMaxHeap`, `jvmGc`, and `cpu`. (see `intermediaries.go` for details and the promQL expressions)

- `GET /api/v2/metrics/series?address=<vm-ip>&metric=<metric>&start=<RFC3339>&end=<RFC3339>`
  - Returns a time series for a metric across the requested interval.
  - If `start` is omitted, it defaults to one hour before now.
  - If `end` is omitted, it defaults to now.
  - The backend requests data with a 15-second step.

Example:

```bash
curl "http://localhost:8080/api/v2/metrics?address=1.1.1.6"
curl "http://localhost:8080/api/v2/metrics/series?address=1.1.1.6&metric=tps&start=2026-08-27T20:00:00Z&end=2026-08-27T21:00:00Z"
```

`PROMETHEUS_PORT` is optional in config, but if it is left at the default value `1111`, metric requests are rejected. The same applies when `PROMETHEUS_API_KEY` is blank.

## Running the app

Place the `.env` with legitimate values in the location as `main.go` and run using make: `make run`

## SSH Config:

```bash
ssh-keygen -t ed25519 -f ./vm_key -C "go-app-user" -N ""
```

Copy the public key to `SSH Keys` in `Instance Metadata` on Compute Engine settings section of a VM.

## Finding the new host key, in case VMs were migrated:

Since on changing the VM, the host key will change, you can find the new host key by running the following command on your local machine, assuming that OpenSSH is installed (scripts to generate are run automatically on Linux)

```bash
ssh-keygen -lf /etc/ssh/ssh_host_ed25519_key.pub -E sha256
```

Also it seems that key carryforward might give unexplained ssh connection issues, so its just better to generate a keypair and update instance metadata accordingly.


## A possible Prometheus deployment

This app can interface with the Prometheus monitoring system to query metrics related to the Minecraft server. You need to set up:

- a metrics exporter on the Minecraft VM that exposes Prometheus-style values
- Prometheus itself, scraping that exporter into its standard API contract
- optionally, a Caddy or Nginx reverse proxy to handle security if you want an API key layer in front of the exporter

These commands below will help you get started:

```bash
# create a prometheus user (no login allowed for security)
sudo useradd --no-create-home --shell /bin/false prometheus

# create directories for configuration and data
sudo mkdir /etc/prometheus
sudo mkdir /var/lib/prometheus

# get prometheus, you can get whatever you want, just make sure it works
cd /tmp
wget https://github.com/prometheus/prometheus/releases/download/v3.14.0/prometheus-3.14.0.linux-amd64.tar.gz

tar xvf prometheus-*.tar.gz
cd prometheus-3.14.0.linux-amd64/

# move binaries to your system path
sudo cp prometheus promtool /usr/local/bin/ 
sudo chown prometheus:prometheus /usr/local/bin/prometheus /usr/local/bin/promtool

# move default configuration files
sudo cp -r consoles/ console_libraries/ /etc/prometheus/ # theses may or may not be present
sudo chown -R prometheus:prometheus /etc/prometheus /var/lib/prometheus
```

Then, edit the Prometheus configuration file to include the Minecraft server scrape job:

```bash
sudo nano /etc/prometheus/prometheus.yml
```

Add the following content:

```yaml
global:
  scrape_interval: 15s

scrape_configs:
  - job_name: 'some-name'
    static_configs:
      - targets: ['localhost:<port>']
```
Then,
```bash
sudo chown prometheus:prometheus /etc/prometheus/prometheus.yml
```

create a systemd service file for Prometheus:

```bash
sudo nano /etc/systemd/system/prometheus.service
```

Add:
```ini
[Unit]
Description=Prometheus Time Series Collection and Processing Server
Wants=network-online.target
After=network-online.target

[Service]
User=prometheus
Group=prometheus
Type=simple
# Notice the --web.listen-address! This ensures Prometheus CANNOT be accessed from the internet directly.
ExecStart=/usr/local/bin/prometheus \
    --config.file /etc/prometheus/prometheus.yml \
    --storage.tsdb.path /var/lib/prometheus/ \
    --web.listen-address=127.0.0.1:9090 \
    --web.console.templates=/etc/prometheus/consoles \
    --web.console.libraries=/etc/prometheus/console_libraries

Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
```
This is fairly barebones.

finally:
```bash
sudo systemctl daemon-reload
sudo systemctl enable prometheus
sudo systemctl start prometheus
```

## ..and a Caddy setup to go with it

```bash
sudo apt install -y debian-keyring debian-archive-keyring apt-transport-https curl
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | sudo gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | sudo tee /etc/apt/sources.list.d/caddy-stable.list
sudo apt update
sudo apt install caddy
```

Then, edit the Caddyfile:

```bash
sudo nano /etc/caddy/Caddyfile
```
to maybe add something like:

```caddy
:9091 {
    # 1. Require the API key
    @denied not header X-API-Key "your-super-secret-api-key-here"
    respond @denied "Unauthorized" 401

    # 2. Proxy to Prometheus
    reverse_proxy 127.0.0.1:9090
}
```

Then, restart Caddy:

```bash
sudo systemctl restart caddy
```
ezpz

## See also- related repos

[the terraform-based orchaestrator](https://github.com/apparentlyarhm/minecraft-terraform)

[the nextJS frontend](https://github.com/apparentlyarhm/minecraft-vm-management-console)
