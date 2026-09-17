# cloudns-sync

`cloudns-sync` discovers locally hosted master DNS zones and synchronizes them to ClouDNS as Secondary/Slave zones. Local zones can be read from BIND/named configuration files or from PowerDNS through its administration command. Discovery and dry-run modes do not modify ClouDNS.

## Run discovery

Use a real configuration file and a copied `named.conf`:

```powershell
go run ./cmd/cloudns-sync discover --config ./config.yml
# short form
go run ./cmd/cloudns-sync discover -c ./config.yml
# discover is also the default command
go run ./cmd/cloudns-sync -c ./config.yml
```

After installing the binary, the equivalent Linux commands are:

```bash
/usr/local/bin/cloudns-sync discover --config /etc/cloudns-sync/config.yml
/usr/local/bin/cloudns-sync --config /etc/cloudns-sync/config.yml
```

The recommended Linux configuration path is `/etc/cloudns-sync/config.yml`. Provider definitions belong in `/etc/cloudns-sync/providers/`. The provider name comes from the `provider:` field inside each file, and `settings.dns_provider` selects which provider to use. Relative provider paths such as `config_file` are resolved from `/etc/cloudns-sync/`. The older `configs/providers/` subdirectory is accepted as a compatibility fallback.

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
| `create` | Compares local zones and creates missing slave zones | When remote comparison is required | With `--apply` only |
| `sync` | Same operation as `create`, intended for repeated runs | When remote comparison is required | With `--apply` only |

`discover` is the default when flags are provided without a command. The presence of `--apply` changes that default to `create`, so both forms are valid:

```bash
/usr/local/bin/cloudns-sync -c /etc/cloudns-sync/config.yml
/usr/local/bin/cloudns-sync discover -c /etc/cloudns-sync/config.yml
/usr/local/bin/cloudns-sync -c /etc/cloudns-sync/config.yml --apply
/usr/local/bin/cloudns-sync create --apply -c /etc/cloudns-sync/config.yml
```

### CLI options

| Option | Commands | Description |
| --- | --- | --- |
| `-c FILE` | all | Short form of `--config` |
| `--config FILE` | all | Main YAML configuration file |
| `--provider NAME` | all | Override `settings.dns_provider` for this run |
| `--named FILE` | all | Override the `named` provider `config_file` |
| `--powerdns-command COMMAND` | all | Override the PowerDNS administration command |
| `--apply` | `create`, `sync` | Enable real ClouDNS create and delete requests; without it the operation is dry-run |
| `--watch` | `create`, `sync` | Repeat synchronization using `settings.sync_interval` |
| `--delete-missing` | `create`, `sync` | Delete remote zones missing locally; only typed Slave zones are eligible |
| `--no-delete-missing` | `create`, `sync` | Disable remote deletion for this run, overriding `settings.delete_missing` |
| `--backend NAME` | all | Deprecated alias for `--provider` |

Useful combinations:

```bash
# Inspect local zones only. No ClouDNS credentials or API request are needed.
/usr/local/bin/cloudns-sync discover -c /etc/cloudns-sync/config.yml

# Test another provider without changing the main YAML.
/usr/local/bin/cloudns-sync discover --provider named -c /etc/cloudns-sync/config.yml

# Preview which missing zones would be created. No API write is performed.
/usr/local/bin/cloudns-sync create -c /etc/cloudns-sync/config.yml

# Create missing zones once. Requires settings.dry_run: false.
/usr/local/bin/cloudns-sync create --apply -c /etc/cloudns-sync/config.yml

# Keep synchronizing. Requires settings.dry_run: false for real writes.
/usr/local/bin/cloudns-sync sync --apply --watch -c /etc/cloudns-sync/config.yml
```

Do not use `--apply` with `settings.dry_run: true`; the program rejects that combination. `settings.delete_missing` is the global switch for deleting remote Slave zones that are missing locally. It is disabled by default and can be overridden for one run with `--delete-missing` or `--no-delete-missing`; the two flags cannot be combined. Actual deletion still requires `--apply`. A deletion dry-run may call the read-only list API to show candidates, but never calls delete. Zones with unknown type, Master type, or matching `exclude_zones` are never deleted. `--watch` requires `--config` so the interval and `run_on_start` settings are available. Press `Ctrl+C` to stop the process.

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

- `cloudns.auth_id`, `cloudns.auth_password`: credentials used by `create` and `sync` when they query or modify remote zones.
- `cloudns.api_url` is an optional API base URL. It defaults to `https://api.cloudns.net`; the client appends the required `/dns/*.json` endpoint for each operation.
- `settings.master_ip`: one or more valid IP addresses. The current create operation uses the first entry.
- `settings.run_on_start`: in watch mode, run one synchronization immediately before waiting for the interval.
- `settings.dry_run`: blocks real writes when `true`.
- `settings.delete_missing`: global switch for deletion of remote Slave zones that are no longer present locally; keep `false` unless explicitly required. Use `--no-delete-missing` to disable deletion for one run even when this is `true`.
- `settings.sync_interval`: Go duration such as `30s`, `1m`, or `1h`; minimum is `1s`.
- `settings.dns_provider`: provider name selected from the provider files.
- `settings.exclude_zones`: normalized names or glob patterns that must not be created remotely. For example, `*.arpa` excludes all reverse-DNS zones.

### Provider file options

Provider files are YAML files under `/etc/cloudns-sync/providers/` or, for compatibility, `/etc/cloudns-sync/configs/providers/`. All YAML files are loaded; the `provider:` field inside each file is the authoritative name.

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
/usr/local/bin/cloudns-sync create -c /etc/cloudns-sync/config.yml
```

To perform the actual registrations:

```bash
/usr/local/bin/cloudns-sync create --apply -c /etc/cloudns-sync/config.yml
```

Before creating, the command calls ClouDNS `dns/list-zones.json` and creates only zones missing from ClouDNS. Existing zones are reported as skipped, not errors. The create request uses `dns/register.json` with `zone-type=slave` and the first `settings.master_ip` value. Use `settings.exclude_zones` for local/system zones that should not be created remotely. The command continues after an individual API failure and prints a final summary. Deletion is controlled globally by `settings.delete_missing`; `--delete-missing` enables it and `--no-delete-missing` disables it for one run. Test first with one dedicated zone and a restricted ClouDNS API user. Credentials are sent only over HTTPS and are never printed.

For continuous synchronization:

```bash
/usr/local/bin/cloudns-sync sync --apply -c /etc/cloudns-sync/config.yml --watch
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

## Linux configuration layout

Use this layout for a system-wide Linux installation:

```text
/usr/local/bin/cloudns-sync
/etc/cloudns-sync/
├── config.yml
└── providers/
    ├── named.yml
    └── powerdns.yml
```

Copy `config.example.yml` to `/etc/cloudns-sync/config.yml` and copy the required definitions from `configs/providers/` to `/etc/cloudns-sync/providers/`. Put the ClouDNS access values under `cloudns.auth_id` and `cloudns.auth_password`, then select a provider with `settings.dns_provider`. For `named`, `config_file` points to the BIND/named configuration. For `powerdns`, `command` and `args` define the local administration command. Replace the documentation-only master IP with one or more real IPv4 or IPv6 addresses. Keep `config.yml` private and use restrictive filesystem permissions because it contains credentials. Credentials are redacted from errors and logs. `sync_interval` uses Go duration syntax and must be at least `1s`.

## Run as a Linux service

The repository includes `deploy/systemd/cloudns-sync.service`. It runs the application as the dedicated `cloudns-sync` user, stores application error logs under `/var/lib/cloudns-sync/logs/`, restarts after failures, and starts after the network is online.

Build the binary and install the application, configuration, and service unit:

```bash
go build -o ./dist/cloudns-sync-linux-amd64 ./cmd/cloudns-sync

sudo groupadd --system cloudns-sync
sudo useradd --system --gid cloudns-sync --home-dir /var/lib/cloudns-sync --shell /usr/sbin/nologin cloudns-sync
sudo usermod -aG named cloudns-sync
sudo systemctl daemon-reload
sudo systemctl restart cloudns-sync.service

sudo install -Dm755 ./dist/cloudns-sync-linux-amd64 /usr/local/bin/cloudns-sync
sudo install -d -o root -g cloudns-sync -m 0750 /etc/cloudns-sync/providers
sudo install -o root -g cloudns-sync -m 0640 config.example.yml /etc/cloudns-sync/config.yml
sudo install -o root -g cloudns-sync -m 0640 configs/providers/*.yml /etc/cloudns-sync/providers/
sudo install -Dm644 deploy/systemd/cloudns-sync.service /etc/systemd/system/cloudns-sync.service
```

Edit `/etc/cloudns-sync/config.yml` and the selected provider file. The service account must be able to read the BIND configuration and all included files, or execute the configured PowerDNS command. Grant only the required group membership or file permissions. Confirm access with a discovery run:

```bash
sudo -u cloudns-sync /usr/local/bin/cloudns-sync discover --config /etc/cloudns-sync/config.yml
```

Test `create` without `--apply` first. The supplied unit uses `--apply`, so set `settings.dry_run: false` only after verifying the discovered zones, master IP, exclusion patterns, and ClouDNS credentials. Then enable the service:

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now cloudns-sync.service
sudo systemctl status cloudns-sync.service
sudo journalctl -u cloudns-sync.service -f
```

After changing the configuration, restart with `sudo systemctl restart cloudns-sync.service`. To disable and stop it, run `sudo systemctl disable --now cloudns-sync.service`.

## Run as a Windows service

The executable is a console application, so do not register it directly with `sc.exe`; Windows would expect the Service Control API and report a startup failure. Use the [WinSW service wrapper](https://github.com/winsw/winsw) with the supplied `deploy/windows/cloudns-sync-service.xml` file.

Use this layout:

```text
C:\Program Files\cloudns-sync\
|-- cloudns-sync.exe
|-- cloudns-sync-service.exe
`-- cloudns-sync-service.xml

C:\ProgramData\cloudns-sync\
|-- config.yml
|-- providers\
|   |-- named.yml
|   `-- powerdns.yml
`-- logs\
```

Build the Windows executable, create the directories, and copy the project files from an elevated PowerShell terminal:

```powershell
go build -o .\dist\cloudns-sync-windows-amd64.exe .\cmd\cloudns-sync

$appDir = Join-Path $env:ProgramFiles "cloudns-sync"
$dataDir = Join-Path $env:ProgramData "cloudns-sync"
New-Item -ItemType Directory -Force $appDir, "$dataDir\providers", "$dataDir\logs" | Out-Null
Copy-Item .\dist\cloudns-sync-windows-amd64.exe "$appDir\cloudns-sync.exe"
Copy-Item .\config.example.yml "$dataDir\config.yml"
Copy-Item .\configs\providers\*.yml "$dataDir\providers\"
Copy-Item .\deploy\windows\cloudns-sync-service.xml "$appDir\cloudns-sync-service.xml"
```

Download the executable from the latest stable [WinSW release](https://github.com/winsw/winsw/releases), rename it to `cloudns-sync-service.exe`, and place it beside the XML file in `C:\Program Files\cloudns-sync\`. WinSW uses the matching base names to associate the executable and XML configuration.

The supplied wrapper runs as the built-in `NetworkService` account. Give that account read access to the configuration and modify access only to the log directory:

```powershell
icacls $dataDir /grant '*S-1-5-20:(OI)(CI)RX'
icacls "$dataDir\logs" /grant '*S-1-5-20:(OI)(CI)M'
```

Edit `C:\ProgramData\cloudns-sync\config.yml` and the selected provider file. Use absolute Windows paths for BIND files, for example `config_file: 'C:\DNS\named.conf'`, and grant `NetworkService` read access to them. The account also needs permission to run and use any configured PowerDNS administration command.

Verify discovery and dry-run behavior interactively before enabling real changes. The wrapper XML uses `--apply`, so set `settings.dry_run: false` only after the configuration is verified:

```powershell
& "$appDir\cloudns-sync.exe" discover --config "$dataDir\config.yml"
& "$appDir\cloudns-sync.exe" create --config "$dataDir\config.yml"
```

Install and start the service from the same elevated PowerShell terminal:

```powershell
& "$appDir\cloudns-sync-service.exe" install
& "$appDir\cloudns-sync-service.exe" start
& "$appDir\cloudns-sync-service.exe" status
```

WinSW writes wrapper, stdout, and stderr logs to `C:\ProgramData\cloudns-sync\logs\`. The service also appears as `cloudns-sync` in `services.msc`. To remove it:

```powershell
& "$appDir\cloudns-sync-service.exe" stop
& "$appDir\cloudns-sync-service.exe" uninstall
```

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
- `cmd/cloudns-sync`: the CLI for discovery, one-time creation, and continuous synchronization.

The BIND parser reads zone declarations and their `type`; it does not read zone files or DNS records. It tolerates ordinary top-level BIND statements, whitespace, comments, quoted zone names, duplicate declarations, and multiline blocks. The PowerDNS source reads only the zone list returned by its command and does not inspect records.

## Plesk and other panels

Plesk is treated as an orchestration layer, not as a separate DNS data format. Configure the provider that Plesk actually uses: `named` with the managed `named.conf`, or `powerdns` with the local PowerDNS command. This avoids coupling discovery to Plesk's internal database schema and also supports non-cPanel BIND installations. New provider types can be added under `internal/dns`, registered in the provider registry, and given their own file under `configs/providers/` without changing the main YAML schema.

## License

This project is licensed under the [MIT License](LICENSE).
