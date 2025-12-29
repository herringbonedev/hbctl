# hbctl — Herringbone Control CLI

`hbctl` is a lightweight control plane for running Herringbone components locally using
Docker Compose. It manages secrets, bootstraps MongoDB, and starts/stops modular services
via simple profiles.

It is designed to feel like a local SOC control plane for Herringbone: simple commands,
modular services, and secure defaults.

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
- `compose.detectionengine.detector.yml`
- `compose.detectionengine.matcher.yml`
- `compose.detectionengine.ruleset.yml`
- `compose.ui.operations-center.yml`

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

hbctl manages services using **profiles**. Each profile maps to a compose file layered
on top of `compose.mongo.yml`.

List available profiles:

```bash
hbctl profiles
```

Wide table view (shows groups):

```bash
hbctl profiles --wide
```

Names only (for scripting):

```bash
hbctl profiles --names
```

Filter:

```bash
hbctl profiles --filter parser
```

JSON output:

```bash
hbctl profiles --json
```

### Available Profiles

| Group      | Profile                    | Description                         |
|------------|----------------------------|-------------------------------------|
| Ingestion  | `logingestion-receiver`    | UDP/TCP/HTTP log ingestion receiver  |
| Core       | `herringbone-logs`         | Logs API service                |
| Parser     | `parser-cardset`           | Cardset metadata parser service      |
| Parser     | `parser-enrichment`        | Log enrichment parser service        |
| Parser     | `parser-extractor`         | Regex/JSONPath extractor service     |
| Detection  | `detectionengine-detector` | Detection engine detector service    |
| Detection  | `detectionengine-matcher`  | Detection engine matcher service     |
| Detection  | `detectionengine-ruleset`  | Detection engine ruleset service     |
| Ops        | `operations-center`        | Operations Center UI / control plane |

MongoDB is automatically started and bootstrapped when required.

---

## Start Services

Start MongoDB only:

```bash
hbctl start --profile database
```

Start receiver (requires type):

```bash
hbctl start --profile logingestion-receiver --type UDP
hbctl start --profile logingestion-receiver --type TCP
hbctl start --profile logingestion-receiver --type HTTP
```

Start other services:

```bash
hbctl start --profile herringbone-logs
hbctl start --profile parser-cardset
hbctl start --profile parser-enrichment
hbctl start --profile parser-extractor
hbctl start --profile detectionengine-detector
hbctl start --profile detectionengine-matcher
hbctl start --profile detectionengine-ruleset
hbctl start --profile operations-center
```

Start the full stack:

```bash
hbctl start --all
```

What happens:
- Decrypts secrets
- Ensures MongoDB is running
- Bootstraps app user if needed
- Starts the requested service(s)

---

## Stop Services

Stop a specific profile:

```bash
hbctl stop --profile logingestion-receiver
hbctl stop --profile herringbone-logs
hbctl stop --profile parser-cardset
hbctl stop --profile parser-enrichment
hbctl stop --profile parser-extractor
hbctl stop --profile detectionengine-detector
hbctl stop --profile detectionengine-matcher
hbctl stop --profile detectionengine-ruleset
hbctl stop --profile operations-center
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
hbctl restart --profile logingestion-receiver
hbctl restart --profile herringbone-logs
hbctl restart --profile parser-cardset
hbctl restart --profile parser-enrichment
hbctl restart --profile parser-extractor
hbctl restart --profile detectionengine-detector
hbctl restart --profile detectionengine-matcher
hbctl restart --profile detectionengine-ruleset
hbctl restart --profile operations-center
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
hbctl status --profile logingestion-receiver
hbctl status --profile herringbone-logs
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
hbctl login mongodb \
  --user hbuser \
  --password hbpass \
  --database herringbone \
  --collection logs \
  --host mongodb

# Start receiver + Mongo
hbctl start --profile logingestion-receiver --type UDP

# Start logs API
hbctl start --profile herringbone-logs

# Start Operations Center UI
hbctl start --profile operations-center

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
- Use `hbctl profiles --wide` to explore available components and groups.

---

## Help

```bash
hbctl help
hbctl <command> --help
```

---

hbctl provides a clean, modular way to operate a local Herringbone SOC stack with
GitOps-style composability and strong operational defaults.
