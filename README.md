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
- `herringbone-auth-e`
- `fingerprint-scoreset-e`
- `fingerprint-identifier-e`
- `parser-enrichment-e`
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

Secrets are written using:

```bash
hbctl login <backend>
```

Supported backends:
- `mongodb`
- `jwtsecret`
- `servicekey`

Secrets are decrypted only at runtime.

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

Start a single unit:

```bash
hbctl start --unit parser
```

Start a single element:

```bash
hbctl start --element fingerprint-scoreset-e
```

Legacy aliases such as `fingerprint-identifier`, `fingerprint-scoreset`, `parser-enrichment`, and `herringbone-auth` are normalized to their enterprise `-e` service names when applicable.

Check status:

```bash
hbctl status
```

View logs:

```bash
hbctl logs --unit parser --follow
```

Stop everything without destroying containers:

```bash
hbctl stop --all
```

`hbctl stop` no longer defaults to the full stack. You must specify `--all`, `--unit`, or `--element`. The default full-stack behavior is now `docker compose stop`, not `docker compose down`. If you intentionally need `down`, make it explicit:

```bash
hbctl stop --all --down
```

Restart a service:

```bash
hbctl restart --element parser-extractor
```

Cleanly upgrade the whole platform without tearing it down:

```bash
hbctl upgrade --all
```

Upgrade one service only:

```bash
hbctl upgrade --element fingerprint-identifier-e
```


## Enterprise Fingerprint Components

This version of hbctl knows about the enterprise fingerprint services:

```text
fingerprint-scoreset-e
fingerprint-identifier-e
parser-enrichment-e
```

The `fingerprint` unit starts score-card management before the identifier:

```bash
hbctl start --unit fingerprint
```

The expected flow is:

```text
fingerprint-scoreset-e -> MongoDB score_cards
fingerprint-identifier-e -> reads/caches score_cards
parser-enrichment-e -> calls fingerprint-identifier-e
```

## Safer Lifecycle Behavior

`hbctl stop` and `hbctl restart` now require explicit scope. This prevents accidentally affecting the whole stack when you meant to operate on one element.

```bash
hbctl stop --element parser-enrichment-e
hbctl stop --unit fingerprint
hbctl stop --all
```

Token generation is no longer part of normal element/unit starts. Service tokens are bootstrapped automatically for `hbctl start --all`, or explicitly when requested:

```bash
hbctl start --element herringbone-auth-e --bootstrap-tokens
```

The old `--no-token-create` flag remains accepted for compatibility, but element and unit starts do not create tokens by default anymore.

## Clean Platform Upgrades

Use `upgrade` instead of `stop` + `start` when refreshing images or recreating services.

```bash
hbctl upgrade --all
hbctl upgrade --unit parser
hbctl upgrade --element fingerprint-scoreset-e
```

Upgrade pulls images by default and recreates containers without running `compose down`. To skip pulling:

```bash
hbctl upgrade --all --no-pull
```

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
hbctl stop
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
