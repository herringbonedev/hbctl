# hbctl

**hbctl** is the control-plane CLI for the Herringbone platform.

It is used to discover, start, stop, restart, and inspect Herringbone services locally using **Docker Compose**, while managing encrypted credentials and enforcing the platform’s **unit / element** model.

hbctl is intentionally opinionated and is the **only supported way** to operate Herringbone locally.

## Repository Relationship

hbctl lives in its **own repository** and operates against a **separate Herringbone checkout**.

Recommended layout:

```text
~/src/herringbone/
  hbctl/
  herringbone/
```

hbctl must be **executed from the Herringbone repository ./docker/ directory**, where `compose.*.yml` files live.

## Core Concepts

### Units
A **unit** is a logical subsystem of the Herringbone platform.

Examples:
- `auth`
- `parser`
- `detection`
- `incidents`
- `search`
- `logs`

### Elements
An **element** is a single deployable service inside a unit.

Examples:
- `herringbone-auth`
- `fingerprint-scoreset`
- `fingerprint-identifier`
- `fingerprint-tuner`
- `parser-enrichment`
- `parser-extractor`
- `detectionengine-detector`
- `operations-center`

hbctl enforces this model consistently across all commands.

## Building hbctl

```bash
go build -o hbctl
```

(Optional) install globally:

```bash
sudo mv hbctl /usr/local/bin/
```

Verify:

```bash
hbctl version
```

## Encrypted Secrets

hbctl stores encrypted credentials locally at:

```text
~/.hbctl/secrets.enc
```

You can move hbctl-managed secrets to a separate location with the global
`--secrets` option:

```bash
hbctl --secrets /secure/herringbone/secrets login mongodb \
  --user herringbone \
  --password 'change-me' \
  --host localhost

hbctl --secrets /secure/herringbone/secrets start --all
```

When `--secrets` is used:

- encrypted hbctl credentials are stored at `<path>/secrets.enc`
- auth runtime secret files are written under `<path>/runtime`
- compose environment variables are exported for compose files that support a
  relocated secrets directory:
  - `HBCTL_SECRETS_DIR`
  - `HB_SECRETS_DIR`
  - `HERRINGBONE_SECRETS_DIR`
  - `RUNTIME_SECRETS_DIR`

When `--secrets` is not used, hbctl keeps the existing behavior.

Authenticate to the Herringbone auth service and store the returned user token in the local hbctl session file:

```bash
hbctl login -u admin@example.com -p 'your-password'
```

By default hbctl tries the proxied auth login route first and then the direct auth microservice login route. You can override the base URL or login path when needed:

```bash
hbctl login -u admin@example.com -p 'your-password' --auth-url http://localhost:8080
hbctl login -u admin@example.com -p 'your-password' --auth-url http://localhost:7001 --login-path /login
```

The user token is saved in a separate session file, normally `~/.hbctl/session.json`, with file mode `0600`. This lets normal CLI commands reuse the token without unlocking `secrets.enc` every time. The token is not printed to the terminal.

Clear the session token with:

```bash
hbctl logout
```

Set `HBCTL_SESSION_FILE` to override the session file path.

Inspect the stored token locally without calling `/me`:

```bash
hbctl whoami
hbctl whoami --json
```

`whoami` decodes the JWT claims from the saved token. It does not require the auth service to expose a `/me` endpoint.

Other secrets are written using:

```bash
hbctl login <backend>
```

Supported backends:
- `mongodb`
- `jwtsecret`
- `servicekey`

Secrets are decrypted only at runtime.

## Release Listing and Upgrade Staging

List published Herringbone releases from GitHub:

```bash
hbctl upgrade --list-releases
```

Limit or machine-read the output:

```bash
hbctl upgrade --list-releases --limit 5
hbctl upgrade --list-releases --json
```

Stage a release into the current compose directory without restarting services:

```bash
hbctl upgrade --release-tag <tag>
```

This downloads the release, archives the current compose directory under `.hbctl/archive/<tag>-<timestamp>`, preserves `secrets/`, and installs the release files.

Stage and immediately apply the release safely:

```bash
hbctl upgrade --release-tag <tag> --now
```

The `--now` path stages the release, replays `init-mongo.js`, applies enterprise platform seed only when `--enterprise` is also supplied, and refreshes application services without removing Docker volumes. If GitHub rate-limits anonymous requests, set `GITHUB_TOKEN` in the environment before running release commands.


## First User Bootstrap

Use `hbctl bootstrap` when a fresh Herringbone auth service has no user yet.

```bash
hbctl bootstrap \
  --email admin@herringbone.dev \
  --password 'test1234' \
  --bootstrap-token-file secrets/runtime/bootstrap_token
```

The command:

1. reads the bootstrap token from the file
2. calls the auth first-user registration endpoint
3. logs in as the created user
4. stores the returned account token in the hbctl session file

If the API server is not available at `localhost:8080`, save the server location first:

```bash
hbctl server set https://your-herringbone.example.com
hbctl bootstrap --email admin@example.com --password 'change-me' --bootstrap-token-file secrets/runtime/bootstrap_token
```

For enterprise, add `--enterprise` to claim the platform org after the user is created and logged in:

```bash
hbctl bootstrap \
  --email admin@herringbone.dev \
  --password 'test1234' \
  --bootstrap-token-file secrets/runtime/bootstrap_token \
  --enterprise
```

You can override paths for older or custom auth deployments:

```bash
hbctl bootstrap \
  --email admin@example.com \
  --password 'change-me' \
  --bootstrap-token-file secrets/runtime/bootstrap_token \
  --server http://localhost:7001 \
  --register-path /register \
  --login-path /login
```

## Common Commands

Discover platform components:

```bash
hbctl units
hbctl elements
```

Start the full platform:

```bash
hbctl start --all
```

`hbctl start --all` now behaves as an idempotent ensure operation:

- if an existing proxy container is present, hbctl starts/reuses it; otherwise hbctl creates proxy
- if an existing MongoDB container or volume is present, hbctl re-attaches safely; otherwise hbctl creates MongoDB
- if an existing auth container is present, hbctl starts/reuses it; otherwise hbctl creates core auth
- enterprise services are skipped unless `--enterprise` is supplied
- before application services start, hbctl ensures every required service account token file exists under `secrets/runtime`
- `--token-create` force-refreshes service tokens, but missing token files are created automatically because Docker bind mounts require them
- after protected core is ready, hbctl creates or starts the remaining application services
- if a dedicated receiver already exists, hbctl reuses it instead of creating a second receiver on the same port

Start the enterprise stack explicitly:

```bash
hbctl start --all --enterprise
```


Start a single unit:

```bash
hbctl start --unit parser
```

Start a single element:

```bash
hbctl start --element parser-extractor
```

Enterprise services require `--enterprise`:

```bash
hbctl start --element fingerprint-scoreset --enterprise
hbctl start --element parser-enrichment --enterprise
```

The `auth` alias always targets the real compose service `herringbone-auth`; `--enterprise` sets enterprise mode without changing the service name.

Check active status:

```bash
hbctl status
```

The default status view is intentionally compact: service, state, replicas, and ports. Include stopped/exited containers only when you explicitly want the full Docker history view:

```bash
hbctl status --all
```

View logs:

```bash
hbctl logs --unit parser --follow
```

Stop application services without touching protected core infrastructure:

```bash
hbctl stop --all
```

`hbctl stop --all` now leaves MongoDB, proxy, and auth running by default. Those services are protected because accidentally stopping them can break access, bootstrap, or database continuity. Stop them only when you explicitly ask for them:

```bash
hbctl stop --proxy
hbctl stop --auth
hbctl stop --mongo
```

`hbctl stop` no longer defaults to the full stack. You must specify `--all`, `--unit`, `--element`, `--proxy`, `--auth`, or `--mongo`. Stopped containers are pruned by default with `docker rm` so `hbctl status` stays clean. Docker volumes are never removed. Use `--keep-containers` if you want the old behavior and prefer to leave exited containers around for inspection.

Restart a service:

```bash
hbctl restart --element parser-extractor
```

Run the safe local refresh without downloading a release:

```bash
hbctl upgrade --all
```

Upgrade one service only:

```bash
hbctl upgrade --element parser-extractor
```

Upgrade enterprise services explicitly:

```bash
hbctl upgrade --element fingerprint-identifier --enterprise
hbctl upgrade --all --enterprise
```


## Enterprise Fingerprint Components

This version of hbctl knows about the enterprise fingerprint services:

```text
fingerprint-scoreset
fingerprint-identifier
fingerprint-tuner
parser-enrichment
```

Enterprise components are not started by default. Their compose service names do not use a `-e` suffix. Use `--enterprise` when starting or upgrading them:

hbctl accepts old `-e` typed aliases for compatibility, but it resolves them to the real compose service names before calling Docker Compose. It also keeps the original enterprise auth-side service identities (`fingerprint-scoreset-e`, `fingerprint-identifier-e`, `fingerprint-tuner-e`, `parser-enrichment-e`) when bootstrapping service accounts, because the containers still advertise those names internally.

```bash
hbctl start --unit fingerprint --enterprise
hbctl start --all --enterprise
```



The expected flow is:

```text
fingerprint-scoreset -> MongoDB score_cards
fingerprint-identifier -> reads/caches score_cards
fingerprint-tuner -> asynchronously proposes or applies score-card improvements for unknown fingerprint events
parser-enrichment -> calls fingerprint-identifier
```


## Alpha 0.6.0 fingerprint-tuner support

`hbctl` now understands the enterprise `fingerprint-tuner` element. It is part of the `fingerprint` unit and is included in enterprise full-stack lifecycle operations when `--enterprise` is used.

Useful commands:

```bash
hbctl start --element fingerprint-tuner --enterprise
hbctl restart --element fingerprint-tuner --enterprise
hbctl logs --follow fingerprint-tuner
hbctl status --unit fingerprint
hbctl upgrade --element fingerprint-tuner --enterprise
```

The tuner keeps the same naming pattern as the other enterprise fingerprint services: the Docker Compose service remains `fingerprint-tuner`, while the auth-side service identity is `fingerprint-tuner-e` and the runtime token file is `fingerprint_tuner_service_token`.

## Safer Lifecycle Behavior

`hbctl stop` and `hbctl restart` now require explicit scope. This prevents accidentally affecting the whole stack when you meant to operate on one element.

```bash
hbctl stop --element parser-enrichment
hbctl stop --unit fingerprint
hbctl stop --all
```

Protected core services are never included in `hbctl stop --all`:

```bash
hbctl stop --proxy
hbctl stop --auth
hbctl stop --mongo
```

After `hbctl stop --all`, `hbctl status` should stay clean because application containers are stopped and then pruned. Protected core containers may still show as running, which is intentional. Dedicated receiver projects are discovered by name/project, stopped, and pruned as application containers. Use `hbctl status --all` only when you intentionally want to inspect stopped containers that were kept or protected.


Prune any old stopped Herringbone containers left behind by previous alpha builds:

```bash
hbctl prune
```

This removes containers only. It never removes Docker volumes. Stopped MongoDB, proxy, and auth containers are skipped unless you explicitly include protected core cleanup:

```bash
hbctl prune --core
```

Service account token files are now lifecycle-managed. When `hbctl start --all` or
`hbctl start --all --enterprise` is about to start services that bind-mount service
tokens, hbctl checks `secrets/runtime` and creates any missing token files before
Docker Compose starts the service. This prevents missing bind-source errors such as
`fingerprint_scoreset_service_token` not existing.

Use `--token-create` when you intentionally want to refresh service tokens even if
the files already exist:

```bash
hbctl start --all --token-create
hbctl start --all --enterprise --token-create
hbctl start --element parser-extractor --token-create
```

The old `--no-token-create` flag is hidden and ignored for compatibility.

## MongoDB Protection

hbctl v0.6.0 treats MongoDB as protected infrastructure.

- `hbctl upgrade --all` does **not** recreate MongoDB.
- `hbctl upgrade --element mongodb` is refused by default.
- `hbctl stop --all` stops application containers only.
- `hbctl stop --all` leaves MongoDB, proxy, and auth running unless you explicitly pass `--mongo`, `--proxy`, or `--auth`.
- `hbctl stop --all` discovers dedicated receiver projects and other Herringbone-owned application containers that are not part of the main compose project.
- `hbctl stop --all --down` is treated as a safe stop and does not remove containers or volumes.
- hbctl stores a stable MongoDB root bootstrap secret in the encrypted hbctl
  secrets file for new local deployments.
- On start, hbctl first checks whether the app MongoDB credentials already work.
  If the DB was merely stopped, it starts MongoDB with `--no-recreate` and waits
  for the existing app user to authenticate.
- If app auth does not work and root auth does not work, hbctl refuses to
  recreate or remove MongoDB data. Fix the stored credentials or recover the
  existing root password instead of deleting the volume.

This is the intended stop/start flow:

```bash
hbctl stop --all
hbctl start --all
```

That flow should re-attach to the existing Docker volume and existing MongoDB
data. If stopped application containers were pruned, `hbctl start --all` creates
them again from the compose files. If a dedicated receiver still exists, hbctl
reuses that receiver instead of creating a duplicate receiver that conflicts on
the same host port.

## Clean Platform Upgrades

Use `upgrade` instead of `stop` + `start` when refreshing images or recreating services.

```bash
hbctl upgrade --all
hbctl upgrade --unit parser
hbctl upgrade --element fingerprint-scoreset
```

Upgrade pulls images by default and recreates containers without running
`compose down`. To preview the full compose plan first:

```bash
hbctl upgrade --all --dry-run
```

Full-stack upgrade behavior in v0.6.0:

1. Validate required core compose files: `compose.mongo.yml`,
   `compose.proxy.yml`, and `compose.herringbone.auth.yml`.
2. Protect MongoDB by excluding it from force-recreate operations.
3. Discover optional compose files from the current directory.
4. Skip optional elements whose compose files are not present.
5. Pull and recreate each non-MongoDB service one at a time.
6. Use `docker compose up -d --no-deps --force-recreate <service>` so dependent
   services and volumes are not torn down.
7. Never run `docker compose down` or remove volumes during upgrade.

To skip pulling:

```bash
hbctl upgrade --all --no-pull
```

To disable forced recreation:

```bash
hbctl upgrade --all --force-recreate=false
```

MongoDB image upgrades are intentionally manual. Back up MongoDB first, then run
the MongoDB upgrade outside hbctl so the operator is forced to make a deliberate
database decision.

## Receiver Note

When starting the log ingestion receiver, a type is required:

```bash
hbctl start --element logingestion-receiver --type UDP
```

## Local Development Notes

### Refreshing images

If you are rebuilding images locally and want to remove stale resources:

```bash
docker system prune -f
```

Then restart:

```bash
hbctl stop --all
hbctl start --all
```

### Resetting MongoDB (local only)

This **permanently deletes local MongoDB data**:

```bash
docker volume rm herringbone_mongo_data
hbctl start --all
```

## Documentation

- Quickstart: https://github.com/herringbonedev/Herringbone/wiki/Quickstart
- hbctl Usage: https://github.com/herringbonedev/Herringbone/wiki/hbctl

## Philosophy

hbctl exists to:
- Make the platform operable, not magical
- Enforce structure over convenience
- Keep runtime logic out of services
- Scale from local development to production-grade orchestration

If something is unclear, it should be fixed in hbctl — not worked around.


## Dedicated Receiver Control Plane

hbctl can run multiple dedicated receiver instances at the same time.
Each receiver gets its own Docker Compose project name based on type and host port.

Start a local receiver with an auto-assigned port:

```bash
hbctl receiver start --type udp --mode local
```

Start a local receiver on a fixed port:

```bash
hbctl receiver start --type tcp --mode local --port 7004
```

Start a forward receiver with an existing key already in hand:

```bash
hbctl receiver start --type udp --mode forward --port 9050   --forward-route 10.0.0.25:7004   --ingestion-key-file ./ingestion.key
```

You may also pass the existing key inline:

```bash
hbctl receiver start --type udp --mode forward --port 9050   --forward-route 10.0.0.25:7004   --ingestion-key hb_ingest_existing_value
```

List dedicated receivers:

```bash
hbctl receiver list
```

Show logs for one receiver:

```bash
hbctl receiver logs --type udp --port 9050 -f
```

Restart one receiver gracefully:

```bash
hbctl receiver restart --type udp --port 9050
```

Stop one receiver:

```bash
hbctl receiver stop --type udp --port 9050
```

Notes:
- hbctl does not mint ingestion keys for receiver forwarding. You supply the existing key value.
- `receiver stop`, `receiver restart`, and `receiver logs` resolve the running container first, then reuse its actual runtime environment so multiple receivers and custom ports remain manageable.
- When `--port` is omitted, hbctl allocates the first free port in the 9000-9999 range for the selected receiver protocol.


## Pretty CLI Output

hbctl v0.6.0 uses consistent operator-friendly output across lifecycle commands.
The CLI prints clear headers, sections, key/value summaries, boxed tables, and
separate labels for `OK`, `WARN`, `SKIP`, `INFO`, `RUN`, and `ERR` messages.

Examples that now render as polished command output:

```bash
hbctl version
hbctl status
hbctl units
hbctl elements --wide
hbctl upgrade --all --dry-run
hbctl start --all --enterprise
hbctl receiver list
```

Disable ANSI color when scripting or writing logs:

```bash
NO_COLOR=1 hbctl status
HBCTL_NO_COLOR=1 hbctl upgrade --all --dry-run
```

#### stop --all hardening note

`hbctl stop --all` performs two phases:

1. Best-effort compose stops for known application services.
2. A final Docker container discovery sweep that stops every running Herringbone
   application container that is not protected core.
3. A safe prune pass that removes stopped application containers with `docker rm`.

This means older alpha compose-file drift, missing optional elements, service-name
changes, scaled replicas, and dedicated receiver projects cannot leave app
containers running just because one compose stop command failed or skipped.
MongoDB, proxy, and auth remain protected during this sweep. hbctl never removes Docker volumes during stop or prune operations.


### MongoDB control-plane checks


MongoDB control-plane note: service containers can still use `MONGO_HOST=mongodb`, but hbctl itself runs on the host and checks/bootstrap MongoDB through `localhost:<port>`. This prevents false `mongo root not ready` failures when the encrypted Mongo secret stores the Docker service name.


### Version revision label

`hbctl version` now includes a small `rev` field so patch-level alpha builds are easy to identify:

```bash
hbctl version
```

Expected output includes:

```text
version  alpha-0.6.0
rev      rev-a3f9c21b7e04
```

This keeps the alpha version stable while still making it obvious that the service-token bootstrap patch is installed.


### rev-20260604-compose-compat-bootstrap

- Service account bootstrap uses the original auth-side enterprise service names while writing the compose-mounted runtime token filenames.
- Enterprise-only gating is based on hbctl logical service metadata, not Docker container names.
- Earlier `_e_service_token` files are treated as legacy read aliases and repaired into the compose-mounted filenames without `_e`.

### Enterprise service naming

`hbctl` does not rename compose services for enterprise mode. Compose service names stay the names present in the compose files, such as `herringbone-auth`, `fingerprint-scoreset`, `fingerprint-identifier`, `fingerprint-tuner`, and `parser-enrichment`. The `--enterprise` flag only sets enterprise-mode environment and includes enterprise compose/image-backed services during start, restart, and upgrade flows.

### Receiver lifecycle note

`hbctl start --all` starts the protected core and application services only. Receivers are intentionally managed separately so each receiver keeps its dedicated compose project, receiver type, and host port:

```bash
hbctl receiver start --type udp --port 7004 --enterprise
hbctl receiver list
hbctl receiver stop --type udp --port 7004
```

This avoids accidentally creating a `logingestion-receiver` inside the main `herringbone` compose project.


## rev-20260604-final-compose-hbctl-fix

- Replays `init-mongo.js` idempotently after MongoDB is reachable so existing volumes still receive platform org/scopes seed data.
- Falls back to direct platform-org seeding if the Mongo init script cannot be replayed.
- Keeps receivers out of `start --all` and removes any receiver container accidentally created in the main project.
- Skips optional services missing from the selected compose/profile set instead of failing the whole start; proxy, MongoDB, and auth still fail loudly.
- Keeps service-account token bootstrap based on the original auth-side identity flow while using the runtime token filenames mounted by the compose files.


### rev-20260604-root-platform-seed

- MongoDB enterprise seed replay now uses root auth inside the `mongodb` container.
- hbctl now always performs a mandatory platform-org upsert after MongoDB is ready, so existing volumes cannot miss `organizations.slug = platform`.
- If `init-mongo.js` replay fails, startup continues with the mandatory platform org seed instead of leaving `/platform/claim` broken.

## Revision label

`hbctl version` now reports an opaque revision hash instead of a descriptive patch name. Release builds inject a random `rev-<hex>` value at build time. Plain `go build` falls back to a short hash of the built binary, so the label remains non-descriptive.

Example:

```text
version  alpha-0.6.0
rev      rev-a3f9c21b7e04
```

### Core/free MongoDB seed behavior

`hbctl start --all` replays the local `init-mongo.js` after MongoDB is reachable so existing MongoDB volumes still get the default org, scopes, and indexes. Enterprise-only platform/org seed data only runs when `--enterprise` is provided.


### Upgrade Mongo seed replay

`hbctl upgrade --all` and `hbctl upgrade --release-tag <tag> --now` replay the common `init-mongo.js` file before refreshing app services. This keeps existing MongoDB volumes current when releases add scopes, indexes, or default seed records. Core/free mode does not seed enterprise platform org data; `hbctl upgrade --all --enterprise` runs that enterprise seed after the common init replay.
