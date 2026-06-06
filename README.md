# hbctl

**hbctl** is the local control CLI for Herringbone Docker Compose deployments.

It starts, stops, upgrades, inspects, and bootstraps Herringbone services while protecting local MongoDB data and managing runtime secrets.

Run hbctl from the Herringbone `docker/` directory, where the `compose.*.yml` files and `init-mongo.js` live.

```text
Herringbone/
  docker/
    compose.*.yml
    init-mongo.js
    secrets/
```

## What hbctl Manages

hbctl manages a local Herringbone stack made of Docker Compose services.

It handles:

- protected core services: proxy, MongoDB, and auth
- application services: parser, search, detection, incidents, operations center, and related services
- service account token bootstrap
- encrypted local hbctl secrets
- common MongoDB seed/init replay
- safe upgrades without deleting Docker volumes
- dedicated receiver lifecycle outside the main stack

Receivers are intentionally managed separately from `start --all` so each receiver can keep its own project, protocol, and port.

## Build and Install

Build locally:

```bash
go build -o hbctl .
```

Install globally:

```bash
sudo cp hbctl /usr/local/bin/hbctl
```

Check the installed version:

```bash
hbctl version
```

`hbctl version` prints the alpha version and an opaque revision value:

```text
version  alpha-0.6.0
rev      rev-a3f9c21b7e04
```

## Where to Run hbctl

Run hbctl from the Herringbone `docker/` directory:

```bash
cd ~/Projects/Herringbone/docker
hbctl status
```

For enterprise:

```bash
cd ~/Projects/Herringbone-enterprise/docker
hbctl status
```

hbctl expects the compose files for the current deployment to be in the current directory.

## Core vs Enterprise

hbctl supports two modes.

### Core / Free

Core mode is the default:

```bash
hbctl start --all
```

Core mode:

- starts the core/free services
- replays the common `init-mongo.js`
- ensures default org, scopes, and indexes
- does not create enterprise platform org data
- does not start enterprise-only services

### Enterprise

Enterprise mode is enabled explicitly:

```bash
hbctl start --all --enterprise
```

Enterprise mode:

- includes enterprise services
- sets enterprise runtime environment
- bootstraps enterprise service account tokens
- replays the common `init-mongo.js`
- ensures enterprise platform/org seed data after the common init

Enterprise mode does **not** rename Docker Compose services. Compose service names stay the names in the compose files, such as:

```text
herringbone-auth
fingerprint-scoreset
fingerprint-identifier
parser-enrichment
```

## Secrets

hbctl stores encrypted CLI-managed secrets in:

```text
~/.hbctl/secrets.enc
```

Runtime secret files used by Docker Compose are written under:

```text
secrets/runtime/
```

Examples include:

```text
bootstrap_token
jwt_secret
service_jwt_private_key
service_jwt_public_key
herringbone_service_token
parser_extractor_service_token
fingerprint_scoreset_service_token
```

Use a separate secrets location with the global `--secrets` option:

```bash
hbctl --secrets /secure/herringbone/secrets start --all
```

When `--secrets` is used:

- encrypted hbctl secrets are stored at `<path>/secrets.enc`
- runtime secret files are written under `<path>/runtime`
- compose-compatible secrets directory environment variables are exported

## Login and Stored User Token

Log in to the Herringbone auth service and store the returned user token in `secrets.enc`:

```bash
hbctl login -u admin@example.com -p 'your-password'
```

Override the auth endpoint if needed:

```bash
hbctl login -u admin@example.com -p 'your-password' \
  --auth-url http://localhost:7001 \
  --login-path /login
```

The token is saved under `auth_token` and is not printed to the terminal.

A useful follow-up command is:

```bash
hbctl whoami
```

Enterprise context:

```bash
hbctl whoami --enterprise
```

JSON output:

```bash
hbctl whoami --json
```

## Start

Start or repair the local stack:

```bash
hbctl start --all
```

Enterprise:

```bash
hbctl start --all --enterprise
```

`start --all` is an idempotent ensure operation:

1. Reuse or create proxy.
2. Reuse or create MongoDB without deleting volumes.
3. Reuse or create auth.
4. Replay the common `init-mongo.js`.
5. In enterprise mode only, ensure enterprise platform/org seed data.
6. Ensure required service account token files exist.
7. Start application services.
8. Leave receivers alone.

Start a unit:

```bash
hbctl start --unit parser
```

Start one element:

```bash
hbctl start --element parser-extractor
```

Start an enterprise element:

```bash
hbctl start --element fingerprint-identifier --enterprise
```

## Stop

Stop application services:

```bash
hbctl stop --all
```

By default, this protects the core infrastructure:

- MongoDB
- proxy
- auth

Stop protected services only when explicitly requested:

```bash
hbctl stop --mongo
hbctl stop --proxy
hbctl stop --auth
```

`stop --all` stops and prunes application containers so `hbctl status` stays clean. It does not remove Docker volumes.

Keep stopped containers for debugging:

```bash
hbctl stop --all --keep-containers
```

## Status

Show a compact status view:

```bash
hbctl status
```

The default view is intentionally small:

```text
SERVICE                  STATE     REPLICAS   PORTS
herringbone-auth         running   1/1        7001
herringbone-search       running   1/1        7014
parser-extractor         running   3/3
```

Include stopped/exited containers only when needed:

```bash
hbctl status --all
```

Machine-readable output:

```bash
hbctl status --json
```

## Logs

View logs for a unit:

```bash
hbctl logs --unit parser
```

Follow logs:

```bash
hbctl logs --unit parser --follow
```

View logs for one element:

```bash
hbctl logs --element parser-extractor --follow
```

## Receivers

Receivers are managed separately from the main stack.

Start a UDP receiver on port 7004:

```bash
hbctl receiver start --type udp --port 7004
```

Enterprise receiver:

```bash
hbctl receiver start --type udp --port 7004 --enterprise
```

List receivers:

```bash
hbctl receiver list
```

Follow receiver logs:

```bash
hbctl receiver logs --type udp --port 7004 --follow
```

Stop a receiver:

```bash
hbctl receiver stop --type udp --port 7004
```

Restart a receiver:

```bash
hbctl receiver restart --type udp --port 7004
```

Receivers use dedicated Docker Compose projects so multiple receivers can run without colliding with the main stack.

## Upgrade

hbctl supports three upgrade workflows.

### List Releases

List available Herringbone releases:

```bash
hbctl upgrade --list-releases
```

Limit results:

```bash
hbctl upgrade --list-releases --limit 5
```

JSON output:

```bash
hbctl upgrade --list-releases --json
```

This only lists releases. It does not compare versions.

### Stage a Release

Download and stage a release without restarting services:

```bash
hbctl upgrade --release-tag <tag>
```

This performs the file upgrade only:

1. Download the release.
2. Extract it under `.hbctl/releases/`.
3. Archive the current compose directory under `.hbctl/archive/<tag>-<timestamp>/`.
4. Preserve:
   - `secrets/`
   - `secrets.enc`
   - `.hbctl/`
5. Install the new release files into the current directory.
6. Do not restart services.

### Stage and Apply a Release

Download, stage, and immediately apply the release:

```bash
hbctl upgrade --release-tag <tag> --now
```

This performs the file upgrade, then runs the safe local upgrade.

Enterprise:

```bash
hbctl upgrade --release-tag <tag> --now --enterprise
```

### Safe Local Upgrade

Refresh the currently installed compose files without downloading a release:

```bash
hbctl upgrade --all
```

Enterprise:

```bash
hbctl upgrade --all --enterprise
```

Upgrade one element only:

```bash
hbctl upgrade --element parser-extractor
```

Enterprise element:

```bash
hbctl upgrade --element fingerprint-identifier --enterprise
```

A safe local upgrade:

1. Ensures MongoDB is reachable.
2. Replays the common `init-mongo.js`.
3. Runs enterprise platform/org seed only when `--enterprise` is used.
4. Ensures required service account token files exist.
5. Pulls images unless `--no-pull` is used.
6. Recreates targeted app service containers.
7. Does not run `docker compose down`.
8. Does not remove Docker volumes.
9. Does not manage receivers.

Preview the plan:

```bash
hbctl upgrade --all --dry-run
```

Skip image pulls:

```bash
hbctl upgrade --all --no-pull
```

## MongoDB Init Behavior

hbctl replays `init-mongo.js` after MongoDB is reachable.

This is intentional. Docker's native `/docker-entrypoint-initdb.d/` hook only runs on a brand-new empty MongoDB volume. hbctl replay allows existing local volumes to receive safe idempotent additions from newer releases, such as:

- new scopes
- updated scope metadata
- default org changes
- new indexes
- other database add-ons

Core/free mode runs the common init only.

Enterprise mode runs the common init and then ensures enterprise platform/org seed data.

## MongoDB Safety Rules

MongoDB is protected infrastructure.

hbctl does not delete MongoDB data.

The following are intentionally avoided during normal lifecycle commands:

```bash
docker compose down -v
docker volume rm
docker rm -v
```

`hbctl stop --all` leaves MongoDB running.

`hbctl upgrade --all` does not recreate MongoDB.

MongoDB upgrades should be handled deliberately by the operator after backup.

## Prune

Remove stopped Herringbone application containers left behind by older alpha builds:

```bash
hbctl prune
```

This does not remove volumes.

Include protected core containers only when explicitly requested:

```bash
hbctl prune --core
```

Even with `--core`, volumes are not removed.

## Common Workflows

Start core/free:

```bash
hbctl start --all
hbctl receiver start --type udp --port 7004
hbctl status
```

Start enterprise:

```bash
hbctl start --all --enterprise
hbctl receiver start --type udp --port 7004 --enterprise
hbctl status
```

Upgrade core/free:

```bash
hbctl upgrade --all
```

Upgrade enterprise:

```bash
hbctl upgrade --all --enterprise
```

Stage a release and apply it later:

```bash
hbctl upgrade --list-releases
hbctl upgrade --release-tag <tag>
hbctl upgrade --all
```

Stage a release and apply it now:

```bash
hbctl upgrade --release-tag <tag> --now
```

Stop application services:

```bash
hbctl stop --all
```

Stop everything including protected core:

```bash
hbctl stop --all
hbctl stop --auth
hbctl stop --proxy
hbctl stop --mongo
```

## Troubleshooting

### Missing token file

If Docker Compose reports a missing `secrets/runtime/*_service_token` file, run:

```bash
hbctl start --all --token-create
```

Enterprise:

```bash
hbctl start --all --enterprise --token-create
```

### Port already allocated

A service with a fixed host port cannot run multiple replicas on the same host.

Use one replica for fixed-port services such as a service publishing `7051:7051`.

### Receiver accidentally created in the main project

Receivers should be managed by `hbctl receiver`, not `start --all`.

Stop the bad main-project receiver and start the dedicated one:

```bash
hbctl stop --element logingestion-receiver
hbctl receiver start --type udp --port 7004
```

### Reset MongoDB for local development only

This permanently deletes local MongoDB data:

```bash
docker volume rm herringbone_mongo_data
hbctl start --all
```

Do not do this on a real deployment.

## Environment

Disable color output:

```bash
NO_COLOR=1 hbctl status
HBCTL_NO_COLOR=1 hbctl upgrade --all --dry-run
```

Use a GitHub token for release listing or release downloads if rate-limited:

```bash
GITHUB_TOKEN=ghp_xxx hbctl upgrade --list-releases
```