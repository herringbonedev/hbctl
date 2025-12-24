# hbctl — Herringbone Control CLI

`hbctl` is a lightweight control plane for running Herringbone components locally using
Docker Compose. It manages secrets, bootstraps MongoDB, and starts/stops modular services
via simple profiles.

---

## Installation

From the hbctl source directory:

```bash
go install
```

Ensure your Go bin is on PATH:

```bash
export PATH="$HOME/go/bin:$PATH"
```

Verify:

```bash
hbctl version
```

---

## Working Directory

Run `hbctl` from the directory that contains your compose files, for example:

```bash
Projects/Herringbone/docker/
```

This directory should contain files like:

- `compose.mongo.yml`
- `compose.logingestion.receiver.yml`
- `compose.herringbone.logs.yml`
- `compose.parser.cardset.yml`
- `compose.parser.enrichment.yml`
- `compose.parser.extractor.yml`

---

## Secrets

hbctl stores MongoDB credentials in an encrypted local file.

Create/update the secret:

```bash
hbctl login mongodb \
  --user <username> \
  --password <password> \
  --database <db> \
  --collection <collection> \
  --host <host> \
  [--port 27017] \
  [--auth-source herringbone]
```

Example (for Docker network use):

```bash
hbctl login mongodb \
  --user hbuser \
  --password hbpass \
  --database herringbone \
  --collection logs \
  --host mongodb
```

You will be prompted for an encryption passphrase.

---

## Profiles

hbctl manages services by profile:

| Profile            | Service                   | Description                         |
|--------------------|---------------------------|-------------------------------------|
| `database`         | `mongodb`                 | MongoDB only                         |
| `receiver`         | `logingestion-receiver`   | Log ingestion receiver               |
| `logs`             | `herringbone-logs`        | Logs API                             |
| `parser-cardset`   | `parser-cardset`          | Cardset parser service               |
| `parser-enrichment`| `parser-enrichment`       | Enrichment parser                    |
| `parser-extractor` | `parser-extractor`        | Extractor parser                     |

MongoDB is automatically started and bootstrapped when required.

---

## Start Services

Start MongoDB only:

```bash
hbctl start --profile database
```

Start receiver (requires type):

```bash
hbctl start --profile receiver --type UDP
hbctl start --profile receiver --type TCP
hbctl start --profile receiver --type HTTP
```

Start other services:

```bash
hbctl start --profile logs
hbctl start --profile parser-cardset
hbctl start --profile parser-enrichment
hbctl start --profile parser-extractor
```

What happens:
- Decrypts secrets
- Ensures MongoDB is running
- Bootstraps app user if needed
- Starts the requested service

---

## Stop Services

Stop a specific profile:

```bash
hbctl stop --profile receiver
hbctl stop --profile logs
hbctl stop --profile parser-cardset
hbctl stop --profile parser-enrichment
hbctl stop --profile parser-extractor
hbctl stop --profile database
```

Stop everything:

```bash
hbctl stop
```

No secrets are decrypted for stop.

---

## Restart Services

Restart a specific profile:

```bash
hbctl restart --profile receiver
hbctl restart --profile logs
hbctl restart --profile parser-cardset
hbctl restart --profile parser-enrichment
hbctl restart --profile parser-extractor
hbctl restart --profile database
```

Restart everything:

```bash
hbctl restart
```

---

## Status

Show status of all running Herringbone containers:

```bash
hbctl status
```

Show status for a single profile:

```bash
hbctl status --profile receiver
hbctl status --profile logs
hbctl status --profile parser-cardset
```

Output includes:

- Container name
- Service
- State
- Published ports

---

## Logs

View logs for a service:

```bash
hbctl logs logingestion-receiver
hbctl logs herringbone-logs
hbctl logs parser-cardset
```

Follow logs:

```bash
hbctl logs --follow logingestion-receiver
```

Tail last N lines:

```bash
hbctl logs --tail 100 herringbone-logs
```

---

## Typical Workflow

```bash
# Save credentials once
hbctl login mongodb --user hbuser --password hbpass --database herringbone --collection logs --host mongodb

# Start receiver + Mongo
hbctl start --profile receiver --type UDP

# Start logs API
hbctl start --profile logs

# Check status
hbctl status

# Follow logs
hbctl logs --follow logingestion-receiver

# Stop everything
hbctl stop
```

---

## Notes

- Always run hbctl from the compose directory.
- Services communicate with Mongo using `MONGO_HOST=mongodb` on the Docker network.
- Secrets are stored encrypted locally; only `start` decrypts them.
- Each profile maps to one compose file layered with `compose.mongo.yml`.

---

## Help

```bash
hbctl help
hbctl <command> --help
```

---

hbctl is designed to feel like a local SOC control plane for Herringbone:
simple commands, modular services, and secure defaults.
