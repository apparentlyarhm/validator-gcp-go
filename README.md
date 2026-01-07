## Introduction

This repository contains the backend service for the GCP Minecraft Control Panel. 
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
* GET /mods [USER/ADMIN]: Lists all mods currently available on the server.
* GET /mods/download/{fileName} [USER/ADMIN]: Provides a download link or stream for a specific mod file.
* GET /logs?address=val&c=count: [USER/ADMIN] Gets a formatted list of logs with fields like Timestamp, level, source and the message it self.
* PATCH /firewall/add-ip: Adds the requesting user's IP address to the firewall whitelist.
* PATCH /firewall/purge: [ADMIN] Removes all IP addresses from the firewall whitelist, essentially preventing all public access.
* PATCH /firewall/make-public: [ADMIN] Opens the server to the public by setting the firewall rule to allow 0.0.0.0/0.

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

**Practically, the amount of priviledge `ANON has is equal to not logging in at all.**

### A note:

when tokens are expired, the server returns 401 instead of 403. the 401 triggers auto login on the frontend.

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
MINECRAFT_SERVER_PORT=value # it is assumed that query port is same as server port
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
```

## Running the app

Place the `.env` with legitimate values in the location as `main.go` and run using make: `make run`

## SSH Config:

```bash
ssh-keygen -t ed25519 -f ./vm_key -C "go-app-user" -N ""
```

Copy the public key to `SSH Keys` in `Instance Metadata` on Compute Engine settings section of a VM.


## See also- related repos

[the terraform-based orchaestrator](https://github.com/apparentlyarhm/minecraft-terraform)

[the nextJS frontend](https://github.com/apparentlyarhm/minecraft-vm-management-console)
