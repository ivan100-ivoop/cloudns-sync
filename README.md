# CloudNS Secondary Zone Scan

Phase 1 discovers authoritative master zones locally through a backend-neutral `ZoneSource`. It does not contact the ClouDNS API and does not create, update, or delete zones.

## Run discovery

Use a real configuration file and a copied `named.conf`:

```powershell
go run ./cmd/cloudns-sync discover --config ./config.yml
# short form
go run ./cmd/cloudns-sync discover -c ./config.yml
# discover is also the default command
go run ./cmd/cloudns-sync -c ./config.yml
```

After building a binary, the equivalent Linux commands are:

```bash
./cloudns-sync-linux-amd64 discover --config /etc/nz/config.yml
./cloudns-sync-linux-amd64 --config /etc/nz/config.yml
```

The configuration can be installed outside the project, for example `/etc/nz/config.yml`. All YAML files in `/etc/nz/providers/` are loaded on each discovery run. The provider name comes from the `provider:` field inside each file, and the selected name is `settings.dns_provider`. The relative `config_file` path is resolved from `/etc/nz`. The older `configs/providers/` layout is accepted as a compatibility fallback.

For parser-only testing, the configuration is optional:

```powershell
go run ./cmd/cloudns-sync discover --provider named --named ./testdata/named.conf
```

PowerDNS can be queried through the local administration command:

```powershell
go run ./cmd/cloudns-sync discover --provider powerdns --powerdns-command pdnsutil
```

The command prints normalized, sorted master zone names. Slave zones are ignored. BIND `include` directives are resolved recursively, including relative paths and glob patterns. Missing include files and cyclic includes fail clearly instead of producing incomplete results.

## Command reference

The compiled binary accepts these commands:

| Command | Behavior | Contacts ClouDNS | Changes zones |
| --- | --- | --- | --- |
| `discover` | Reads and prints local master zones | No | No |
| `create` | Compares local zones and creates missing slave zones | With `--apply` | With `--apply` only |
| `sync` | Same operation as `create`, intended for repeated runs | With `--apply` | With `--apply` only |

`discover` is the default when flags are provided without a command. The presence of `--apply` changes that default to `create`, so both forms are valid:

```bash
./cloudns-sync-linux-amd64 -c /etc/nz/config.yml
./cloudns-sync-linux-amd64 discover -c /etc/nz/config.yml
./cloudns-sync-linux-amd64 -c /etc/nz/config.yml --apply
./cloudns-sync-linux-amd64 create --apply -c /etc/nz/config.yml
```

### CLI options

| Option | Commands | Description |
| --- | --- | --- |
| `-c FILE` | all | Short form of `--config` |
| `--config FILE` | all | Main YAML configuration file |
| `--provider NAME` | all | Override `settings.dns_provider` for this run |
| `--named FILE` | all | Override the `named` provider `config_file` |
| `--powerdns-command COMMAND` | all | Override the PowerDNS administration command |
| `--apply` | `create`, `sync` | Enable real ClouDNS create requests; without it the operation is dry-run |
| `--watch` | `create`, `sync` | Repeat synchronization using `settings.sync_interval` |
| `--delete-missing` | `create`, `sync` | Delete remote zones missing locally; only typed Slave zones are eligible |
| `--backend NAME` | all | Deprecated alias for `--provider` |

Useful combinations:

```bash
# Inspect local zones only. No ClouDNS credentials or API request are needed.
./cloudns-sync-linux-amd64 discover -c /etc/nz/config.yml

# Test another provider without changing the main YAML.
./cloudns-sync-linux-amd64 discover --provider named -c /etc/nz/config.yml

# Preview which missing zones would be created. No API write is performed.
./cloudns-sync-linux-amd64 create -c /etc/nz/config.yml

# Create missing zones once. Requires settings.dry_run: false.
./cloudns-sync-linux-amd64 create --apply -c /etc/nz/config.yml

# Keep synchronizing. Requires settings.dry_run: false for real writes.
./cloudns-sync-linux-amd64 sync --apply --watch -c /etc/nz/config.yml
```

Do not use `--apply` with `settings.dry_run: true`; the program rejects that combination. Deletion is disabled by default and requires both `--apply` and `--delete-missing` (or `settings.delete_missing: true`). A deletion dry-run may call the read-only list API to show candidates, but never calls delete. Zones with unknown type, Master type, or matching `exclude_zones` are never deleted. `--watch` requires `--config` so the interval and `run_on_start` settings are available. Press `Ctrl+C` to stop the process.

### Main config options

```yaml
cloudns:
	auth_id: "..."
	auth_password: "..."

settings:
	master_ip:
		- "203.0.113.10"
	run_on_start: true
	dry_run: true
	delete_missing: false
	sync_interval: 1m
	dns_provider: named
	exclude_zones:
		- "localhost"
```

- `cloudns.auth_id`, `cloudns.auth_password`: credentials used only by `create`/`sync --apply`.
- `cloudns.api_url` is not required. If it is absent, the binary uses the standard ClouDNS register endpoint and derives the list endpoint automatically.
- `settings.master_ip`: one or more valid IP addresses. The current create operation uses the first entry.
- `settings.run_on_start`: in watch mode, run one synchronization immediately before waiting for the interval.
- `settings.dry_run`: blocks real writes when `true`.
- `settings.delete_missing`: enables deletion of remote Slave zones that are no longer present locally; keep `false` unless explicitly required.
- `settings.sync_interval`: Go duration such as `30s`, `1m`, or `1h`; minimum is `1s`.
- `settings.dns_provider`: provider name selected from the provider files.
- `settings.exclude_zones`: normalized names or glob patterns that must not be created remotely. For example, `*.arpa` excludes all reverse-DNS zones.

### Provider file options

Provider files are YAML files under `/etc/nz/providers/` or, for compatibility, `/etc/nz/configs/providers/`. All YAML files are loaded; the `provider:` field inside each file is the authoritative name.

Named/BIND:

```yaml
provider: named
type: bind
config_file: "/etc/named.conf"
```

PowerDNS:

```yaml
provider: powerdns
type: command
command: "pdnsutil"
args:
	- "list-all-zones"
```

`named` reads BIND zone declarations and includes. `powerdns` runs the configured local command and expects one zone name per output line. The provider directory must contain a file whose internal `provider:` matches `settings.dns_provider`.

## Test ClouDNS slave-zone creation

The `create` command discovers local master zones and prepares ClouDNS Secondary/Slave zone registrations. It is a dry run unless `--apply` is explicitly supplied:

```bash
./cloudns-sync-linux-amd64 create -c ./dev-config/config.yml
```

To perform the actual registrations:

```bash
./cloudns-sync-linux-amd64 create --apply -c ./dev-config/config.yml
```

Before creating, the command calls ClouDNS `dns/list-zones.json` and creates only zones missing from ClouDNS. Existing zones are reported as skipped, not errors. The create request uses `dns/register.json` with `zone-type=slave` and the first `settings.master_ip` value. Use `settings.exclude_zones` for local/system zones that should not be created remotely. The command continues after an individual API failure and prints a final summary. This phase only creates zones; it never deletes or updates zones. Test first with one dedicated zone and a restricted ClouDNS API user. Credentials are sent only over HTTPS and are never printed.

For continuous synchronization:

```bash
./cloudns-sync-linux-amd64 sync --apply -c ./dev-config/config.yml --watch
```

The current binary accepts `create --watch` as well. With `settings.run_on_start: true`, synchronization runs immediately and then repeats using `settings.sync_interval`. Stop it with `Ctrl+C`.

## Build

The project is pure Go and can be compiled for Windows and Linux. Cross-compilation does not contact DNS or ClouDNS; it only creates the executable.

### Build on Windows for Windows

Run in PowerShell from the project directory:

```powershell
New-Item -ItemType Directory -Force dist | Out-Null
go build -o .\dist\cloudns-sync-windows-amd64.exe .\cmd\cloudns-sync
```

For Windows ARM64:

```powershell
$env:GOOS = "windows"
$env:GOARCH = "arm64"
go build -o .\dist\cloudns-sync-windows-arm64.exe .\cmd\cloudns-sync
Remove-Item Env:GOOS
Remove-Item Env:GOARCH
```

### Build on Linux for Linux

```bash
mkdir -p dist
go build -o ./dist/cloudns-sync-linux-amd64 ./cmd/cloudns-sync
```

For Linux ARM64:

```bash
GOOS=linux GOARCH=arm64 go build -o ./dist/cloudns-sync-linux-arm64 ./cmd/cloudns-sync
```

### Cross-compile on Windows for Linux

```powershell
New-Item -ItemType Directory -Force dist | Out-Null
$env:GOOS = "linux"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "0"
go build -o .\dist\cloudns-sync-linux-amd64 .\cmd\cloudns-sync
Remove-Item Env:GOOS
Remove-Item Env:GOARCH
Remove-Item Env:CGO_ENABLED
```

### Cross-compile on Linux for Windows

```bash
mkdir -p dist
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o ./dist/cloudns-sync-windows-amd64.exe ./cmd/cloudns-sync
```

### Cross-compile on Linux for Linux and Windows ARM64

```bash
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o ./dist/cloudns-sync-linux-arm64 ./cmd/cloudns-sync
GOOS=windows GOARCH=arm64 CGO_ENABLED=0 go build -o ./dist/cloudns-sync-windows-arm64.exe ./cmd/cloudns-sync
```

`CGO_ENABLED=0` is recommended for portable cross-compiled binaries. The application can run on Windows, but the selected data source must also be available there: `named` needs a readable local BIND configuration file, while `powerdns` needs the configured administration command such as `pdnsutil`. A Linux `/etc/named.conf` is not automatically available on Windows; copy it to a Windows path and point the provider file to that path.

## Configuration

Copy `config.example.yml` to `config.yml` and put the CloudNS access values directly under `cloudns.auth_id` and `cloudns.auth_password`. Select a provider with `settings.dns_provider`. Provider files live under `providers/`; their `provider:` field is authoritative. For `named`, `config_file` points to the BIND/named configuration. For `powerdns`, `command` and `args` define the local administration command. Replace the documentation-only master IP with one or more real valid IPv4/IPv6 addresses. Never commit `config.yml`; it is ignored by Git and should have restrictive filesystem permissions. Credentials are redacted from errors and logs. `sync_interval` uses Go duration syntax and must be at least `1s`.

## Verification

```powershell
go test ./...
go vet ./...
```

## Architecture

- `internal/config`: YAML loading, credential fields, duration and IP validation.
- `internal/dns`: the `ZoneSource` abstraction shared by all DNS providers.
- `internal/dns/providers`: provider registry and provider-specific configuration selection.
- `internal/dns/bind`: quote-aware comment stripping and master-zone discovery.
- `internal/dns/powerdns`: PowerDNS zone discovery through a configured local command.
- `internal/logger`: centralized sensitive-value redaction and date-based error files.
- `cmd/cloudns-sync`: the local `discover` CLI only.

The BIND parser reads zone declarations and their `type`; it does not read zone files or DNS records. It tolerates ordinary top-level BIND statements, whitespace, comments, quoted zone names, duplicate declarations, and multiline blocks. The PowerDNS source reads only the zone list returned by its command and does not inspect records.

## Plesk and other panels

Plesk is treated as an orchestration layer, not as a separate DNS data format. Configure the provider that Plesk actually uses: `named` with the managed `named.conf`, or `powerdns` with the local PowerDNS command. This avoids coupling discovery to Plesk's internal database schema and also supports non-cPanel BIND installations. New provider types can be added under `internal/dns`, registered in the provider registry, and given their own file under `configs/providers/` without changing the main YAML schema.

## Next compatibility input

Provide a redacted copy of the real cPanel `/etc/named.conf` and all files it includes, preserving the same relative layout or absolute paths. The parser supports ordinary BIND views, block comments, nested includes, and glob patterns. Also provide the intended valid master IP list and a sample production logging configuration. Do not include CloudNS passwords, tokens, or authorization headers.