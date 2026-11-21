# gceTools

[![build](https://github.com/jordilopezr/gcetools/actions/workflows/build.yml/badge.svg)](https://github.com/jordilopezr/gcetools/actions/workflows/build.yml) · License GPL-3.0 · [gcetools.jordilopezr.com](https://gcetools.jordilopezr.com) · **[Español](README.md)**

An RVTools-style inventory for Google Compute Engine: a desktop app for Windows, Linux and macOS, plus a command-line mode.
It reads every project the identity can reach and exports to XLSX (one sheet per tab), CSV (ZIP) or JSON.

- **Read-only:** it only requests the `cloud-platform.read-only` scope.
- **Authentication:** Sign in with Google without gcloud (session kept in the system keyring), gcloud credentials (ADC), a service account JSON key (kept in memory only) or service account impersonation.
- **Languages:** English and Spanish (UI, inventory, report and CLI).
- **Stack:** Go + [Wails v2](https://wails.io) + React/TypeScript, with a Material 3 UI and AG Grid Community.

## Tabs

| Tab | RVTools | Contents |
|---|---|---|
| gInfo | vInfo | One row per VM: machine type, vCPU, RAM, status, IPs, OS, disks, service account, labels, Shielded VM, MIG, managing service (GKE, Dataproc…) |
| gCPU / gMemory | vCPU / vMemory | CPU and memory details |
| gDisk | vDisk | Disks attached to each VM |
| gNetwork | vNetwork | NICs, alias IPs, external IP, VPC project (Shared VPC) |
| gDisks | vDatastore | Every disk, orphaned ones included |
| gSnapshot | vSnapshot | Snapshots with age and stored size |
| gLicense | vLicense | OS and SQL Server licenses per disk |
| gMIG / gTemplate | vCluster | MIGs, autoscaling, health checks, templates |
| gVPC / gSubnet / gFirewall / gAddress | vSwitch / dvPort | Networking |
| gImage | vCD | Custom images and machine images, with age and deprecation |
| gReservation / gCommitment | — | Reservations (with % unused) and CUDs (days until they end) |
| gSoleTenant | vHost | Sole-tenant nodes and the VMs they host |
| gQuota | — | Regional quotas with usage > 0 and % used |
| gTools * | vTools | Actual OS, kernel and agent version (OS Config) |
| gPerf * | — | CPU and RAM: average, p95 and maximum over N days (Cloud Monitoring; RAM needs the Ops Agent) |
| gRecommend * | — | Active recommendations: rightsizing, idle resources and CUDs, with monthly savings |
| gCost * | — | Estimated monthly cost per project: compute and GPU, licenses, disks, snapshots, images, unused IPs |
| gProject | vSource | Per-project summary and scan result |
| gHealth | vHealth | Findings: orphaned disks, firewall open to the Internet, default service account, stopped VMs, disks without a snapshot schedule, templates with deprecated or old images… |
| gErrors / gMetaData | vMetaData | Errors per project and scan information |

\* Optional modules. Turn them on in the Projects screen or with `--modules`. If the API is disabled or the permission is missing, the module is skipped: the project shows as "OK (without …)" and the reason is logged in gErrors.

### Sign in with Google

To use gceTools without installing the Google Cloud CLI: **Connection → Sign in with Google**. Your browser opens, you grant access and the session is saved; next time you open the app you are already connected.

- It only requests `cloud-platform.read-only` (plus your email, to show the account). It uses the OAuth flow for installed apps with PKCE and a redirect to `127.0.0.1`.
- The refresh token is stored **only in the system keyring** (Credential Manager on Windows, Keychain on macOS, Secret Service on Linux), never in files. "Sign out" deletes it; you can also revoke access from [your Google Account](https://myaccount.google.com/permissions).
- gceTools does not send your data to any server of its own: it talks to Google APIs directly. See [PRIVACY.md](PRIVACY.md).
- From the command line: `gcetools-cli login`, then `gcetools-cli scan --auth oauth`; `gcetools-cli logout` signs out.

**If your organization blocks third-party apps** (Google Workspace or Cloud Identity policies), there are two ways out:

- An administrator allows gceTools in *Admin console → Security → Access and data control → API controls*.
- Or use your own OAuth client: in a project of your organization, set up *APIs & Services → OAuth consent screen* as **Internal**, and create *Credentials → OAuth client ID* of type **Desktop app**. Share the JSON file; each user loads it in **Connection → Advanced options → Use my organization's OAuth client** (or `gcetools-cli login --client client_secret_….json`). With your own client, the quota of the optional APIs (Recommender, Cloud Asset, Cloud Billing) is charged to that project, which must have them enabled.

Builds made from source do not include the gceTools client (it is injected when releases are published); only your own client works in them.

### Language

The UI is available in English and Spanish: the selector is in the top bar (translate icon), and the system language is used by default.

- Scans, XLSX/CSV exports and the report are produced in the active language: tab and column titles, health rules, statuses and metadata. Sheet names (`gInfo`, `gDisks`…) never change, so scripts that read the files keep working.
- When you open a scan from the history or a JSON snapshot, headers are shown in the active language; finding texts stay in the language the scan ran in.
- From the command line: `--lang en|es` (or the `GCETOOLS_LANG` variable); without it, the system language is used.

### VMs managed by GKE, Dataproc, Dataflow or Batch

gInfo shows in **Managed by** which service created each VM (GKE, Dataproc, Dataflow, Batch or MIG) and, for GKE and Dataproc, the **cluster** and **node pool**. It is detected from the labels and metadata those services set.

- Health rules that make no sense for disposable VMs are skipped for them: stopped VM, stopped VM without disks, deletion protection, snapshot schedule and Ops Agent. The rest (public IP, default service account, Secure Boot…) still apply.
- In **Inventory**, the **Hide the N VMs managed by GKE, Dataproc, Dataflow or Batch** checkbox removes GKE, Dataproc, Dataflow and Batch nodes from the per-VM tabs (gInfo, gCPU, gMemory, gDisk, gNetwork, gPerf, gTools, gLicense and gHealth). Members of your own MIGs stay visible.

### Fast mode with Cloud Asset Inventory

In organizations with many projects, **fast mode** (Projects → **Fast mode (Cloud Asset Inventory)**, or `--fast`) reads VMs, disks, snapshots, images, machine images, templates, MIGs, autoscalers, VPCs, subnets, firewalls and IPs from Cloud Asset Inventory instead of the Compute API:

- **Without a scope:** one listing per project instead of a dozen calls.
- **With an organization or folder** in the discovery scope (or `--discover`): a single paged listing for every project.
- Reservations, commitments, sole-tenant nodes, quotas and the optional modules (OS Config, Monitoring, Recommender) are still read per project, because CAI does not include them or lacks that data.
- **Delay:** CAI can take a few minutes to reflect recent changes; gMetaData records that the scan used fast mode.
- If CAI fails for a project (permissions, disabled API), that project is read through the Compute API and the reason is logged in gErrors.
- It needs `roles/cloudasset.viewer` on the scope and the Cloud Asset API enabled in the quota project (or in the project of the service account or OAuth client).

```bash
gcetools-cli scan --discover organizations/123456 --fast --out inventory.xlsx
```

### History and trends

Every scan run from the app is saved automatically (you can turn it off in **History**) as compressed JSON in the user data folder: `~/.local/share/gcetools/history` on Linux, `%LocalAppData%\gcetools\history` on Windows and `~/Library/Application Support/gcetools/history` on macOS. The 100 most recent are kept.

- **History** lists the scans and charts the evolution of VMs, vCPU, cost (actual when available, otherwise estimated) and high-severity findings.
- **Open in Inventory** loads an older scan; **Compare with previous** (or selecting two) opens the comparison without exporting anything by hand.
- From the command line: `gcetools-cli scan --save-history` also saves to the same history (useful for scheduled tasks).

### Executive report

**Inventory → Export → Executive report (HTML)** produces a self-contained page (no scripts or external resources) that opens in the browser and prints to PDF: totals, estimated or actual cost, Recommender savings, the top 10 findings, cost per project, projects that could not be read and, when there is history, the changes since the previous scan. From the command line: `gcetools-cli scan --report report.html`.

### Profiles and discovery with Cloud Asset Inventory

- **Profiles:** in Projects → **Save profile** you store the filter, selected projects, modules, currency, parallelism, discovery scope and authentication method. Apply them from the "Profile" selector or with `--profile`. They hold no secrets: for a service account key only the file path is saved. They live in `profiles.json` inside the user config folder (`~/.config/gcetools` on Linux, `%AppData%\gcetools` on Windows, `~/Library/Application Support/gcetools` on macOS), with 0600 permissions.
- **Detect projects with Compute:** a single Cloud Asset Inventory search over an organization or folder (`organizations/123`, `folders/456`) returns which projects have VMs, disks, snapshots, images, templates, reservations or commitments, and selects only those. In organizations with hundreds or thousands of projects it avoids scanning those that don't use Compute Engine. It needs `roles/cloudasset.viewer` on that scope and the Cloud Asset API enabled in the quota project (or in the service account's project).

```bash
gcetools-cli scan --discover organizations/123456 --out inventory.json
gcetools-cli scan --profile production --out inventory.xlsx
gcetools-cli profiles
```

### Compare inventories

Export each scan as a **JSON snapshot** and compare them on the **Compare** page (no Google Cloud connection needed) or with `gcetools-cli diff`. The result has five tabs:

| Tab | Contents |
|---|---|
| Summary | Additions, removals and changes per resource type |
| Added / Removed | New and deleted resources, with a summary of each |
| Changes | One field per row: old value and new value |
| Metadata | Compared inventories, monthly cost difference and what could not be compared |

- **Relevant first:** resized VMs, service account or Shielded VM changes, new public IPs, firewall rules open to the Internet, disks that become orphaned, new high-severity findings and commitment changes.
- **No false positives from permissions:** a resource is only compared if its collector worked in both scans; if a project failed in one of them it shows in Metadata as "Not compared" instead of hundreds of removals.
- **No noise:** last start and stop times, ages, quota usage, metrics and snapshots created by a schedule (they rotate daily) are ignored. Costs are only compared when both inventories use the same currency.

```bash
gcetools-cli diff --out changes.xlsx previous.json new.json
gcetools-cli diff --fail-on-relevant baseline.json today.json   # exit code 3 when there are relevant changes
```

### Configurable health rules

Every gHealth rule has a stable ID (`gcetools-cli rules` lists them). In **Projects → Health rules** you can turn rules off and tune their thresholds (days stopped, snapshot and image age, CPU, RAM, quotas, CUD end date); the configuration is saved in the profile.

To exempt specific resources without turning a rule off, use labels on the VM, disk, template or project:

| Label | Effect |
|---|---|
| `gcetools-ignore=<id>` | Skips that rule for the resource |
| `gcetools-ignore-<id>=true` | Same, and lets you combine several rules |
| `gcetools-ignore=all` | Skips every rule for the resource |

For example, a project labeled `gcetools-ignore=snapshot-schedule` raises no findings about disks without a snapshot schedule (useful when backups are done by Veeam or Backup and DR). gMetaData shows how many findings were hidden and which rules were off.

```bash
gcetools-cli rules
gcetools-cli scan --disable-rules snapshot-schedule,no-labels
```

### Actual cost (BigQuery Billing Export)

Optional. With [Cloud Billing export to BigQuery](https://cloud.google.com/billing/docs/how-to/export-data-bigquery) enabled, gceTools reads the **net** Compute Engine cost for a month (with CUDs, SUDs, credits and discounts) and puts it next to the list-price estimate:

- **gCostReal:** per project, gross cost, credits, net, list-price estimate and difference. With the **detailed** export (`gcp_billing_export_resource_v1_…`) it separates what is attributed to VMs and disks from what is not (network/egress, IPs, snapshots, commitments, resources already deleted).
- **gInfo and gDisks:** an "Actual cost/month (BigQuery)" column per VM and per disk (detailed export only).
- **Query cost:** this is the only gceTools feature that can cost money, because BigQuery charges for data read (1 TiB free per month per billing account). gceTools filters by the month's partitions, runs a free estimate (*dry run*) first and does not run the query if it would read more than the limit (50 GB by default). gMetaData shows the GB read.
- **Permissions:** `roles/bigquery.jobUser` on the table's project and `roles/bigquery.dataViewer` on the dataset.

```bash
gcetools-cli scan --billing-table finops-prj.billing.gcp_billing_export_resource_v1_XXXXXX_XXXXXX_XXXXXX --billing-month 202610
```

### Estimated cost

The `pricing` module downloads the public Compute Engine catalog (Cloud Billing Catalog API). It caches it for 24 hours in `~/.cache/gcetools/` or its Windows and macOS equivalent, and computes on-demand list prices at 730 h/month in the chosen currency (`--currency USD|EUR|CLP…`). It adds cost columns to gInfo, gDisks, gSnapshot, gImage, gAddress and gProject, and produces the gCost tab.

- **Running VM:** compute + GPU + licenses + disks.
- **Stopped VM:** disks only.
- **Not included:** sustained use discounts (SUDs), CUDs, negotiated prices, network/egress and BYOL/BYOS licenses.
- **Priced paid licenses:** Windows Server, SQL Server, RHEL (also for SAP), SLES (also for SAP) and Ubuntu Pro (also FIPS and Minimal Pro; by vCPU tier plus RAM). Container-Optimized OS and free distributions have no license cost.
- **Anything without a price** shows in gInfo ("Cost notes" column) and in gMetaData ("Not priced").
- **Validation against the real catalog:**

  ```bash
  GCETOOLS_CATALOG=~/.cache/gcetools/compute-skus-USD.json go test ./internal/pricing -run Real -v
  ```

## Installation

| System | Package | Requirements |
|---|---|---|
| Windows 10/11 (x64 or ARM64) | `gcetools-…-setup.exe` (installer) or portable `.zip` | WebView2 Runtime (already included in up-to-date Windows 10/11; if missing, the app offers to install it) |
| macOS 11+ (Intel and Apple Silicon) | `.dmg` (drag to Applications) or `.zip` with `gcetools-cli`; universal binary | — |
| Debian 12+ / Ubuntu 22.04+ (x64) | `.deb` | Dependencies resolved by `apt` |
| Fedora 36+ / RHEL 9+ / openSUSE (x64) | `.rpm` | Dependencies resolved by `dnf` / `zypper` |
| Any x64 Linux | `.AppImage` or `.tar.gz` | GTK 3 and WebKitGTK 4.1 installed (they ship with GNOME on current distros) |

Every package also includes `gcetools-cli`, the console version for scheduled tasks.

On Linux:

```bash
sudo apt install ./gcetools-0.7.0-linux-amd64.deb
```

```bash
sudo dnf install ./gcetools-0.7.0-linux-x86_64.rpm
```

```bash
chmod +x gcetools-0.7.0-linux-x86_64.AppImage && ./gcetools-0.7.0-linux-x86_64.AppImage
```

The `.deb` and `.rpm` install `gcetools` and `gcetools-cli` in `/usr/bin` and add gceTools to the applications menu. The AppImage does not bundle GTK or WebKitGTK: it uses the system ones.

**While the binaries are not signed:**
- **Windows:** SmartScreen warns about an "unknown publisher". Click "More info", then "Run anyway".
- **macOS:** Gatekeeper blocks the app. Open it with right-click → Open, or remove the quarantine flag:

  ```bash
  xattr -dr com.apple.quarantine gceTools.app
  ```

**Signing:**
- **macOS:** the workflow signs and notarizes only when the `APPLE_CERT_P12`, `APPLE_CERT_PASSWORD`, `APPLE_SIGN_IDENTITY`, `APPLE_ID`, `APPLE_TEAM_ID` and `APPLE_APP_PASSWORD` secrets exist. It needs an Apple Developer account (USD 99/year).
- **Windows:** free signing for open source projects from [SignPath Foundation](https://signpath.org) (see [CODE_SIGNING_POLICY.md](CODE_SIGNING_POLICY.md)). Only releases (`v*` tags) are signed, and only when the `SIGNPATH_API_TOKEN` secret and the `SIGNPATH_ORGANIZATION_ID`, `SIGNPATH_PROJECT_SLUG` and `SIGNPATH_SIGNING_POLICY_SLUG` variables exist. Signing runs in two rounds: first `gcetools.exe` and `gcetools-cli.exe`, then the installer, rebuilt with the signed executable. SignPath artifact configurations live in `.signpath/artifact-configurations/`. Each signing request is approved by hand in SignPath. Windows shows "SignPath Foundation" as the publisher.

## Cross-platform builds

The [.github/workflows/build.yml](.github/workflows/build.yml) workflow runs the tests and builds on native runners:

| Platform | Output |
|---|---|
| Linux | `.deb`, `.rpm`, `.AppImage` and `.tar.gz` ([build/linux/package.sh](build/linux/package.sh), with nfpm and appimagetool) |
| Windows | `.zip` and NSIS installer |
| macOS | `.dmg` and `.zip` with the universal `.app` |

Each build is published as an artifact. Pushing a `v*` tag also creates a GitHub Release.

From Linux you can also build for Windows locally, because Wails does not need CGO on Windows:

```bash
wails build -platform windows/amd64,windows/arm64
GOOS=windows CGO_ENABLED=0 go build -o build/bin/gcetools-cli.exe ./cmd/gcetools-cli
GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -o build/bin/gcetools-cli-macos ./cmd/gcetools-cli
```

The macOS GUI app can only be built on a Mac (or the `macos-latest` runner), because it needs Apple's SDK.

## Development requirements

- Go 1.23+ and Node 22+ (CI uses Node 26)
- Wails CLI: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`
- System dependencies (`wails doctor` checks them):
  - **Linux:** `libgtk-3-dev` and `libwebkit2gtk-4.1-dev` (Debian 13 / Ubuntu 24.04 only ship 4.1, hence `-tags webkit2_41`)
  - **Windows:** WebView2 (ships with Windows 10/11)
  - **macOS:** Xcode Command Line Tools

## Usage

```bash
wails dev -tags webkit2_41                 # app with hot reload
wails build -tags webkit2_41               # binary in build/bin/ (Linux)
wails build -platform windows/amd64        # on Windows
wails build -platform darwin/universal     # on macOS
go test ./...                              # backend tests
```

To work on the UI only, without Wails or credentials:

```bash
npm --prefix frontend run dev              # opens http://localhost:5173 with mock data ("Demo mode")
```

### Command line (scheduled tasks)

```bash
gcetools-cli scan --out inventory.xlsx
gcetools-cli scan --auth sa --key sa.json --query "parent:folders/123456" --out inv.zip --csv-sep ";"
gcetools-cli scan --auth impersonate --impersonate inventory@prj.iam.gserviceaccount.com --projects a,b
gcetools-cli scan --out "inventory-$(date +%F).json"      # snapshot to compare later
gcetools-cli diff --out changes.xlsx previous.json new.json
gcetools-cli scan --profile production --discover organizations/123456
gcetools-cli profiles
gcetools-cli scan --save-history --report "report-$(date +%F).html"   # history + report for the weekly task
gcetools-cli login && gcetools-cli scan --auth oauth
gcetools-cli scan --discover organizations/123456 --fast           # fast mode with Cloud Asset Inventory
gcetools-cli --lang es scan --out inventario.xlsx                   # inventory in Spanish
gcetools-cli version
```

`gcetools-cli` ships in every package and works the same on Windows, macOS and Linux. On Linux and macOS the app binary also accepts `gcetools scan …` and `gcetools diff …`; on Windows it does not, because the GUI app does not write to the console.

## API usage cost

gceTools only reads. Almost every API it uses is free; Cloud Monitoring can bill, with a free tier that in practice is never used up, and so can BigQuery if you turn on actual cost.

| API | Used for | Cost |
|---|---|---|
| Compute Engine | Inventory of VMs, disks, networking, snapshots… | Free (per-minute request quotas only) |
| Resource Manager | Listing projects | Free |
| Cloud Billing Catalog | Prices for the estimated cost (24 h cache) | Free |
| Recommender | Recommendations | Free |
| Cloud Asset Inventory | Finding projects with Compute and fast mode | Free (per-minute quotas per project) |
| Cloud Monitoring | CPU and RAM metrics (gPerf) | USD 0.50 per million series returned; the first million each month per billing account is free |
| BigQuery | Actual cost (optional, off by default) | Per data read (1 TiB free per month); gceTools estimates first and respects a GB limit |
| OS Config | OS and agents (gTools) | Reading is free. VM Manager charges per VM with an active agent (USD 0.003/h, 100 VMs free per billing account), but that depends on having the agent, not on gceTools |
| GitHub | Checking for updates | Free |

For reference, a scan requests about 2 Monitoring series per VM (CPU and RAM): with 100 VMs you would need about 5,000 scans a month to leave the free tier. With user credentials, anything billable goes to the ADC quota project. Prices as of November 2026: see [Monitoring](https://cloud.google.com/monitoring/pricing), [VM Manager](https://cloud.google.com/compute/vm-manager/pricing) and [Cloud Asset Inventory](https://cloud.google.com/asset-inventory/pricing).

## IAM permissions

- Minimum: `roles/compute.viewer` and `roles/browser` (at organization or folder level for a global view).
- gTools: `roles/osconfig.inventoryViewer`, with the OS Config API enabled in the project and the agent on the VMs.
- gPerf: `roles/monitoring.viewer`.
- Discovery and fast mode with Cloud Asset Inventory: `roles/cloudasset.viewer` on the organization, folder or project.
- Actual cost: `roles/bigquery.jobUser` on the billing table's project and `roles/bigquery.dataViewer` on its dataset.
- gRecommend: `roles/recommender.computeViewer`. With user credentials Google requires a quota project: the ADC `quota_project_id` is used (`gcloud auth application-default set-quota-project <project>`), which must have the Recommender API enabled.
- Without `resourcemanager.projects.list` you can add projects by ID by hand.

## Layout

```
main.go, app.go           Wails startup + methods exposed to the frontend
internal/auth             ADC / JSON key / impersonation / Sign in with Google
internal/core             Project discovery, concurrent scan, rate-limited HTTP with retries
internal/collect          One collector per resource type (aggregatedList or Cloud Asset Inventory) + tab schema
internal/health           gHealth rules
internal/diff             Comparison of two inventories (additions, removals and changes)
internal/pricing          Cloud Billing catalog and estimated cost
internal/export           XLSX (excelize streaming), CSV in ZIP, JSON
internal/cli              `scan`, `diff`, `profiles`, `rules`, `login` and `version` commands
internal/profile          Saved scan profiles
internal/billing          Actual cost from the Cloud Billing export to BigQuery
internal/history          Local scan history
internal/report           Executive report in HTML
internal/i18n             Translations (the Spanish text is the key; English in en_*.go)
frontend/                 React + TS, Material 3 tokens, AG Grid
assets/brand/             Visual identity: master SVGs, per-platform icons, favicon and web assets
```

To add a tab: define the sheet in `internal/collect/schema.go`, write its collector and register it in `Project()` (collect.go). The UI and the exporters show it with no further changes.

To translate: wrap the text with `i18n.T("…")` or `i18n.Tf("…", args)` in Go and with `t('…')` in the frontend, and add the English version to `internal/i18n/en_*.go` or `frontend/src/lib/i18n.en.ts`. `go test ./internal/i18n` and `npm --prefix frontend run check:i18n` fail when one is missing.

## Roadmap

- Windows executable signing
- Security posture tab (gSecurity)

## Updates

At startup the app checks GitHub for a newer release; you can turn the check off in "About". From the command line: `gcetools-cli version`. The check is an anonymous GET to GitHub's public API and sends no user or inventory data.

## Releasing

1. Update the version in `internal/core/core.go`, `wails.json` and `frontend/package.json`. CI injects the tag's version into the binary anyway.
2. Create and push the tag:

   ```bash
   git tag v0.2.0
   git push origin v0.2.0
   ```

3. The workflow publishes the GitHub Release with the Linux, Windows and macOS packages.

## Support the project

gceTools is free. If you find it useful, you can [buy me a coffee](https://buymeacoffee.com/jordimlopezr).

## License

Copyright © 2026 Jordi Lopez Reyes.

This program is free software: you can redistribute it and/or modify it under the terms of the **GNU General Public License v3.0** as published by the Free Software Foundation. It is distributed in the hope that it will be useful, but **without any warranty**. See the [LICENSE](LICENSE) file for details.
