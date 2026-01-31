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
hbctl start --element herringbone-auth
```

Check status:

```bash
hbctl status
```

View logs:

```bash
hbctl logs --unit parser --follow
```

Stop everything:

```bash
hbctl stop
```

Restart a service:

```bash
hbctl restart --element parser-extractor
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
