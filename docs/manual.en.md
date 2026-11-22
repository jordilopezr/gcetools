# gceTools manual

*Version 0.7 · [Español](manual.md)*

gceTools takes a complete inventory of Google Compute Engine, in the style of RVTools for VMware. It reads every project your identity can reach and presents the information in tabs: VMs, disks, networks, snapshots, costs, best-practice findings… From there you can export to Excel, CSV, JSON or an HTML report. There is a desktop application for Windows, macOS and Linux, and a command-line version for scheduled tasks.

This manual explains every screen, every option and every command.

## Contents

1. [Basic concepts](#1-basic-concepts)
2. [Installation](#2-installation)
3. [Getting started](#3-getting-started)
4. [The application window](#4-the-application-window)
5. [Connection](#5-connection)
6. [Projects and scanning](#6-projects-and-scanning)
7. [Inventory](#7-inventory)
8. [Tab reference](#8-tab-reference)
9. [Health rules (gHealth)](#9-health-rules-ghealth)
10. [Costs](#10-costs)
11. [Compare inventories](#11-compare-inventories)
12. [History](#12-history)
13. [Executive report](#13-executive-report)
14. [About and updates](#14-about-and-updates)
15. [Language](#15-language)
16. [Command line](#16-command-line)
17. [IAM permissions and APIs](#17-iam-permissions-and-apis)
18. [API usage cost](#18-api-usage-cost)
19. [Local files and data](#19-local-files-and-data)
20. [Privacy and security](#20-privacy-and-security)
21. [Troubleshooting](#21-troubleshooting)
22. [Frequently asked questions](#22-frequently-asked-questions)
23. [RVTools equivalents](#23-rvtools-equivalents)

---

## 1. Basic concepts

- **Read-only.** gceTools only requests the `cloud-platform.read-only` scope. It doesn't create, modify or delete anything in Google Cloud.
- **Identity.** The inventory includes whatever the identity you connect with can see: your Google account, your gcloud credentials or a service account. Anything it can't see (because of IAM permissions) doesn't appear, and gceTools reports it in the gErrors tab.
- **Scan.** Reading the selected projects. Each scan produces an **inventory**: a set of tabs (gInfo, gDisks, gHealth…) plus the scan metadata.
- **Optional modules.** Besides the Compute API, gceTools can read OS Config (the VMs' actual OS), Cloud Monitoring (CPU and RAM), Recommender (savings recommendations), the pricing catalog (estimated cost) and the Cloud Billing export to BigQuery (actual cost). Each one is turned on separately.
- **JSON snapshot.** An inventory saved as a JSON file. Use it to compare two scans or to open it later.
- **History.** The app automatically saves every scan on your computer, so you can see trends and compare without exporting anything.
- **Nothing leaves your computer.** gceTools has no servers: it talks directly to the Google APIs and stores the results on your computer.

## 2. Installation

Download the package for your system from the [releases page](https://github.com/jordilopezr/gcetools/releases/latest) or from [gcetools.jordilopezr.com](https://gcetools.jordilopezr.com).

| System | Package | Requirements |
|---|---|---|
| Windows 10/11 (x64 or ARM64) | `gcetools-<version>-windows-amd64-setup.exe` (installer) or portable `.zip` | WebView2 Runtime (included in up-to-date Windows 10/11; if it's missing, the app offers to install it) |
| macOS 11 or later (Intel and Apple Silicon) | `.dmg` or `.zip` (universal binary) | — |
| Debian 12+ / Ubuntu 22.04+ | `.deb` | `apt` resolves the dependencies |
| Fedora 36+ / RHEL 9+ / openSUSE | `.rpm` | `dnf` / `zypper` resolves the dependencies |
| Any Linux x64 | `.AppImage` or `.tar.gz` | GTK 3 and WebKitGTK 4.1 installed |

Every package includes the application (`gcetools`) and the console version (`gcetools-cli`).

**Windows.** Run the installer or unzip the `.zip` wherever you like. While the executables are unsigned, SmartScreen warns about an "Unknown publisher": click **More info → Run anyway**.

**macOS.** Open the `.dmg` and drag gceTools to Applications. Gatekeeper blocks unsigned apps: open it the first time with **right-click → Open**, or remove the quarantine attribute:

```bash
xattr -dr com.apple.quarantine /Applications/gceTools.app
```

**Debian / Ubuntu:**

```bash
sudo apt install ./gcetools-0.8.0-linux-amd64.deb
```

**Fedora / RHEL / openSUSE:**

```bash
sudo dnf install ./gcetools-0.8.0-linux-x86_64.rpm
```

**AppImage:**

```bash
chmod +x gcetools-0.8.0-linux-x86_64.AppImage && ./gcetools-0.8.0-linux-x86_64.AppImage
```

The `.deb` and `.rpm` packages install `gcetools` and `gcetools-cli` in `/usr/bin` and add gceTools to the applications menu.

**Uninstall.** Like any other application (Control Panel on Windows, Trash on macOS, `sudo apt remove gcetools` or `sudo dnf remove gcetools` on Linux). Local data (history, profiles, pricing cache and the keyring session) isn't removed automatically: it's described in [Local files and data](#19-local-files-and-data).

## 3. Getting started

1. Open gceTools. The **Connection** screen appears.
2. Choose **Sign in with Google** and click the button: your browser opens, you pick your account and accept the read-only permission. If you already use gcloud, you can choose **gcloud credentials** instead.
3. Click **Connect**. The app moves to **Projects** and lists the projects your account can see.
4. Select the projects you want to inventory (the header checkbox selects them all) and click **Scan**.
5. When it finishes, **Inventory** opens. Check the figures in the header, the **gHealth** tab with the findings and, if you like, **Export → Excel (.xlsx)**.

For your account, `roles/compute.viewer` and `roles/browser` on the projects, folders or organization are enough. The optional modules need a few more roles: see [IAM permissions and APIs](#17-iam-permissions-and-apis).

## 4. The application window

**Top bar**, from left to right:

- **Logo and name.**
- **Version X available:** appears when there's a newer release. It takes you to **About**, which has the release notes and the download link.
- **Demo mode:** only appears if the interface runs outside the app (in a browser, for development), with simulated data.
- **Theme** (moon or sun icon): System default, Light or Dark.
- **Language** (translate icon): Español or English. See [Language](#15-language).
- **Account:** with an active session, it shows the identity. Its menu shows the authentication method and lets you **Disconnect**.

**Navigation rail:**

| Section | Used for | Available |
|---|---|---|
| Connection | Choose an identity and connect | Always |
| Projects | Choose what to scan and with which options | With an active session |
| Inventory | View and export the latest inventory | When an inventory is loaded |
| Compare | Compare two inventories | Always |
| History | Previous scans and trends | Always |
| About | Version, updates, API costs, licenses | Always (at the bottom) |

Short messages (scan completed, file exported, errors) appear in a temporary bar at the bottom.

## 5. Connection

Choose the identity used to read the inventory. What you see depends on that identity's IAM permissions. There are four methods:

### 5.1 Sign in with Google

The recommended option: you don't need to install gcloud.

1. Select **Sign in with Google** and click the button of the same name.
2. Your browser opens the Google sign-in screen. Pick the account and accept. gceTools only requests `cloud-platform.read-only` and your email (to show which account you signed in with).
3. The browser shows "Signed in"; you can close that tab. The app connects on its own.

The session (a refresh token) is stored **in the system keyring**: Credential Manager on Windows, Keychain on macOS or Secret Service (GNOME Keyring, KWallet) on Linux. When you reopen gceTools you're already connected: **Session saved** appears with your account.

- **Sign out** removes the token from the keyring. You can also revoke access from [your Google Account](https://myaccount.google.com/permissions).
- **Cancel**, while waiting for the browser, aborts the sign-in.
- Technically, it's the OAuth flow for installed apps, with PKCE and a redirect to `127.0.0.1`.

**Advanced options: your organization's OAuth client.** Some Google Workspace or Cloud Identity organizations block external apps that request access to Google Cloud. They have two ways out:

- **Allow gceTools:** an administrator allows it in *Admin console → Security → Access and data control → API controls*.
- **Use your own client:** an administrator creates, in an organization project, an OAuth consent screen of type **Internal** and an *OAuth client ID* of type **Desktop app**, and distributes the JSON. Each user loads it in **Advanced options → Use my organization's OAuth client**.
  - **Back to the gceTools client** stops using your own client.
  - With your own client, API quota is charged to that project, which must have the APIs enabled (see [APIs to enable](#173-apis-to-enable)).

> Until Google finishes verifying gceTools, signing in with the built-in client only works for the test users added by the maintainer, and the session expires every 7 days. The other methods work without that limitation.

**Keyring unavailable.** If the system has no keyring service (for example, a Linux system without GNOME Keyring or KWallet), the card says so and you need to use another method. gceTools never stores the token in a file.

### 5.2 gcloud credentials (ADC)

Uses the Google Cloud CLI Application Default Credentials. If they're missing, the screen shows **Not found** and the command to run once:

```bash
gcloud auth application-default login
```

Reopen the Connection screen and **Detected** appears with your account.

The ADC **quota project** is the project Google charges for the APIs that require one, such as Recommender or Cloud Asset Inventory. Set it with:

```bash
gcloud auth application-default set-quota-project <project>
```

### 5.3 Service account (JSON)

**Select JSON file** loads a service account key. Its email, its project and the beginning of the key ID are shown. The key is **only kept in memory**: it isn't copied or saved, and it's discarded when you disconnect or close the app.

### 5.4 Impersonate service account

Your account gets short-lived tokens for a service account, without downloading keys. This is ideal if your organization prohibits keys.

- Enter the service account email.
- Your account needs `roles/iam.serviceAccountTokenCreator` on it.
- Requires ADC credentials (`gcloud auth application-default login`).

### 5.5 Connect

**Connect** validates the credentials and moves to **Projects**. If there was already a session, connecting again replaces it. The **Recommended IAM permissions** panel summarizes the required roles.

## 6. Projects and scanning

### 6.1 The project list

On entering, every active project the identity can see is listed, with its project ID, name, number and parent (folder or organization).

- **Resource Manager filter** (at the top): narrows the search using Resource Manager syntax and is applied with **Search**. Examples:
  - `parent:folders/123456`: projects directly under a folder.
  - `labels.env:prod`: projects with the label `env=prod`.
  - `name:erp*`: name starting with "erp".
- **Filter the list:** instantly filters the projects already loaded, by any text.
- **Add IDs manually:** type project IDs separated by commas or spaces and press Enter. Useful when the identity isn't allowed to list projects (`resourcemanager.projects.list`) but can read them.
- **Selection:** select the projects to scan; the header checkbox selects all visible ones. The counter shows visible and selected projects.
- The table can be sorted, filtered by column and resized.

### 6.2 Detect projects with Compute (Cloud Asset Inventory)

In large organizations many projects don't use Compute Engine. Enter a scope (`organizations/123456` or `folders/456`) and click **Detect projects with Compute**.

- A single Cloud Asset Inventory query returns which projects have VMs, disks, snapshots, images, templates, reservations or commitments.
- Only those are selected, and the table gains columns with what was found in each one: VMs (CAI), Disks and Other.
- Requires `roles/cloudasset.viewer` on that scope and the Cloud Asset API enabled in the quota project.

### 6.3 Profiles

A profile saves a scan configuration so you can repeat it with one click (or with `--profile` in the CLI):

- the filter, the selected projects and the detection scope;
- the modules, metrics days, currency, fast mode and actual cost;
- the health rules and their thresholds;
- the parallelism and the authentication method (for the CLI).

**Save profile** asks for a name; if it already exists, it's overwritten. The **Profile** selector applies a saved profile, and the trash icon deletes it after confirmation.

Profiles **don't store secrets**: for a service account key, only the file path is saved. They're stored in `profiles.json`, in the user's configuration folder.

### 6.4 Optional modules

| Module | What it adds | Notes |
|---|---|---|
| **OS and agents (OS Config)** | gTools tab: actual OS, version, kernel, hostname, agent version | Needs the OS Config API in the project and the agent on the VMs |
| **CPU/RAM metrics** | gPerf tab: average, p95 and maximum CPU and RAM; rules cpu-low, cpu-high, mem-high, no-ops-agent | Days selector: 7, 14, 30 or 42. RAM requires the Ops Agent |
| **Recommendations** | gRecommend tab: rightsizing, idle resources and CUDs, with monthly savings | With user credentials, requires a quota project with the Recommender API |
| **Estimated cost** | Cost columns in several tabs and the gCost tab | Currency selector: USD, EUR, CLP, MXN, COP, BRL, GBP |

If a module's API is disabled in a project or a permission is missing, the module is skipped **without marking the project as an error**. The project shows as "OK (without osconfig)" or similar, and the reason is recorded in gErrors.

### 6.5 Fast mode (Cloud Asset Inventory)

The **Fast mode (Cloud Asset Inventory)** button changes how VMs, disks, snapshots, images, machine images, templates, MIGs, autoscalers, VPCs, subnets, firewall rules and IPs are read:

- **Without a detection scope:** one Cloud Asset Inventory query per project, instead of a dozen Compute API calls.
- **With an organization or folder as the detection scope:** a single paginated query for all projects.

When to use it: in scans of hundreds of projects, or when the Compute API is throttling requests.

What you need to know:

- **Still read per project:** reservations, commitments, sole-tenant nodes, quotas, project metadata (OS Login, SSH keys) and the optional modules, because CAI doesn't include them or doesn't have that data.
- **Delay:** CAI can take a few minutes to reflect recent changes. gMetaData shows that the scan used fast mode.
- **Fallback:** if CAI fails in a project (permissions, API disabled), that project is read with the Compute API and the reason is recorded in gErrors.
- **Permissions:** `roles/cloudasset.viewer` on the scope and the Cloud Asset API enabled in the quota project, in the service account's project or in the OAuth client's project.

### 6.6 Actual cost (BigQuery)

The **Actual cost (BigQuery)** button expands the actual cost settings. Full explanation in [Costs](#102-actual-cost-bigquery).

- **Billing export table:** `project.dataset.table`.
- **Month:** the month before the scan, or a specific one.
- **Limit of GB read:** 50 GB by default.
- **Turn off** clears the table and disables the module.

### 6.7 Health rules

The **Health rules** button expands the list of the 28 rules, with their severity, category, ID and where the label exception is read.

- Clear a rule's checkbox to **disable it** for this scan and in the profile.
- Rules with a threshold have a field to adjust it: days stopped, age of snapshots and images, % of low or saturated CPU, % of RAM, % of quota and days until a CUD expires.
- **Defaults** restores the original configuration.

Details of each rule in [Health rules](#9-health-rules-ghealth).

### 6.8 Parallelism and scanning

- **Parallelism:** how many projects are read at the same time (8 by default). More parallelism finishes sooner but makes more requests per second. gceTools rate-limits requests (20 per second) and retries transient errors.
- **Scan (N):** starts scanning the N selected projects.

During the scan you see:

- a progress bar with finished projects, VMs found and errors;
- what is being read at each moment ("Reading disks in …");
- a log with the result for each project.

**Cancel** stops the scan and shows what was read up to that point, marked as **Partial**.

When it finishes, the app opens **Inventory** and, if history is turned on, saves the scan to it.

**Result per project** ("Scan result" column in gProject):

| Result | Meaning |
|---|---|
| OK | Everything was read |
| OK (without osconfig, monitoring…) | Read; some optional module wasn't available |
| Partial (N errors) | Some resource types couldn't be read (see gErrors) |
| Compute API disabled | The project doesn't have the Compute API enabled |
| Permission denied | The identity can't read Compute in that project |
| Not found | The project doesn't exist or isn't visible |
| Cancelled / Not scanned | The scan was cancelled before reaching it |

## 7. Inventory

### 7.1 Header and figures

The header shows the scan date, the projects (how many OK), the duration and the identity. **Partial** indicates a cancelled scan.

The figure cards:

- **VMs** and **Running**.
- **vCPU**, **RAM (GB)** and **Total disk (GB)**.
- **Orphaned disks**, **High findings** and **Internet exposed** (VMs with sensitive ports open, see gSecurity): highlighted when greater than zero.
- **List cost/month**, if estimated cost was turned on.
- **Actual cost/month**, if BigQuery was turned on.
- **Estimated savings/month**, if recommendations were turned on.

### 7.2 Managed VMs

If the inventory has VMs created by GKE, Dataproc, Dataflow or Batch, the **Hide the N VMs managed by GKE, Dataproc, Dataflow or Batch** checkbox appears.

- **What it hides:** those VMs in the per-VM tabs (gInfo, gCPU, gMemory, gDisk, gNetwork, gPerf, gTools, gLicense, gHealth and gSecurity).
- **What it doesn't hide:** members of your own MIGs, which remain visible.
- The preference is remembered between sessions.
- **Export** always includes all VMs.

### 7.3 Tabs and table

- **Tab chips:** each one shows the number of rows. Hover over it to see the description.
- **Search in …:** filters the rows of the active tab by any text.
- **Table:** sort by clicking a header, filter by column with the funnel icon and resize by dragging the edge. Cell text can be selected and copied.
- **Special cells:** statuses and severities are shown as colored tags; Yes/No checkboxes as text.
- **Formats:** dates in `YYYY-MM-DD HH:MM` format; GB with 2 decimals; amounts with 2 decimals in the scan currency.

### 7.4 Export

The **Export** button offers:

| Format | Contents |
|---|---|
| **Excel (.xlsx)** | A workbook with one sheet per tab, frozen header, autofilters and adjusted widths |
| **CSV (comma)** | A ZIP with one CSV per tab |
| **CSV (semicolon)** | The same, separated by `;`: this is what Excel expects with Spanish regional settings |
| **Executive report (HTML)** | One-page summary, printable to PDF (see [Executive report](#13-executive-report)) |
| **Snapshot JSON** | The complete inventory, to compare it or open it later |

The suggested name is `gcetools-YYYYMMDD-HHMM` with the scan date. Headers and texts are exported in the active language; sheet names (`gInfo`…) are always the same.

Exported files and the report can only be read by your user (0600 permissions on Linux and macOS): they contain IPs, service accounts and Internet exposure. In CSV files, text starting with `=`, `+`, `-` or `@` gets a leading apostrophe so Excel does not run it as a formula; XLSX does not need it.

## 8. Tab reference

Each tab is a table. Tabs that depend on an optional module or on cost only have data if that module was turned on.

#### gInfo — Instances

One row per VM (equivalent to vInfo).

Project · VM · ID · Zone · Region · Status · Machine type · Family · vCPU · RAM (GB) · CPU platform · Provisioning · Host maintenance · Auto restart · Created · Last start · Last stop · Internal IP · External IP · VPC · Subnet · OS (license) · Image · Disk count · Boot disk (GB) · Total disks (GB) · GPUs · Service account · Scopes · Labels · Network tags · Deletion protection · Secure Boot · vTPM · Confidential · MIG · Managed by · Cluster (GKE/Dataproc) · GKE node pool · Hostname · Self link · Compute+GPU/month (if running) · Licenses/month · Disks/month · Current cost/month · Cost notes · Actual cost/month (BigQuery)

- **Provisioning:** STANDARD, SPOT or PREEMPTIBLE.
- **OS (license):** the system inferred from the boot disk license; if there isn't one, the source image. The actual OS reported by the agent is in gTools.
- **Managed by:** GKE, Dataproc, Dataflow, Batch or MIG, based on the labels and metadata each service sets. For GKE and Dataproc the cluster is filled in and, for GKE, the node pool too.
- **Cost columns:** "Compute+GPU/month" is what it would cost running the whole month; "Current cost/month" is compute plus licenses plus disks if it's running, and only disks if it's stopped.

#### gCPU — CPU

CPU details per VM (vCPU).

Project · VM · Zone · Machine type · vCPU · Threads/core · Visible cores · CPU platform · Min CPU platform · Custom · Shared core · Nested virt.

#### gMemory — Memory

Memory details per VM (vMemory).

Project · VM · Zone · Machine type · RAM (GB) · GB/vCPU · Custom · Extended memory

#### gDisk — VM disks

Disks attached to each VM (vDisk).

Project · VM · Zone · Device · Disk · Boot · Auto delete · Mode · Interface · Type · Size (GB) · Provisioned IOPS · Provisioned MB/s · Encryption · Source image · Licenses

**Encryption** can be Google (Google-owned keys), CMEK (Cloud KMS) or CSEK (customer-supplied key).

#### gNetwork — NICs

Network interfaces per VM (vNetwork).

Project · VM · Zone · NIC · NIC type · VPC · VPC project · Subnet · Internal IP · Alias IP · External IP · Tier · Stack · IPv6 · IP forwarding

**VPC project** differs from the VM's project when Shared VPC is used.

#### gDisks — All disks

Disk inventory, orphaned disks included (vDatastore).

Project · Disk · Location · Type · Size (GB) · Status · VMs · Orphaned · Created · Last attach · Last detach · Provisioned IOPS · Provisioned MB/s · Encryption · Source image · Source snapshot · Snapshot schedule · Replicas · Architecture · Labels · Cost/month · Actual cost/month (BigQuery)

- **Orphaned:** the disk isn't attached to any VM.
- **Snapshot schedule:** the disk's snapshot schedule resource policies.
- **Replicas:** the zones of regional disks.

#### gSnapshot — Snapshots

Disk snapshots (vSnapshot).

Project · Snapshot · Source disk · Disk size (GB) · Stored (GB) · Status · Created · Age (days) · Type · Scheduled · Location · Encryption · Labels · Cost/month

**Stored (GB)** is the space the snapshot actually takes up, and what is billed. **Scheduled** indicates whether it was created by a snapshot schedule.

#### gLicense — Licenses

OS and software licenses on disks (vLicense).

Project · VM · Zone · Disk · Category · License · License project

**Category** groups the licenses: Windows, SQL Server, RHEL, SLES, Ubuntu Pro, Ubuntu, Debian, Rocky Linux, CentOS, Container-Optimized OS, Tool (licenses added by migration tools) or Other.

#### gMIG — MIGs

Managed instance groups and autoscaling (vCluster).

Project · MIG · Location · Scope · Template · Target size · Stable · Autoscaling · Mode · Min · Max · CPU target · Health check · Update policy · Base instance name

#### gTemplate — Templates

Instance templates.

Project · Template · Created · Machine type · Disk type · Disk (GB) · Image · Image project · VPC · Service account · Provisioning · Labels

If the image is a family, it's shown as `family/<name>`.

#### gVPC — VPCs

VPC networks (vSwitch).

Project · VPC · Subnet mode · Routing · MTU · Subnet count · Peerings · Created · Description

#### gSubnet — Subnets

Subnets (dvPort).

Project · Subnet · Region · VPC · CIDR · Secondary ranges · Gateway · Private Google Access · Flow logs · Purpose · Stack · IPv6

#### gFirewall — Firewall

VPC firewall rules.

Project · Rule · VPC · Direction · Priority · Action · Protocols/ports · Sources · Source tags/SA · Destinations · Target tags/SA · Disabled · Logging · Open to the internet

**Open to the internet** flags enabled ingress rules that allow traffic from `0.0.0.0/0` or `::/0`.

#### gAddress — IPs

Reserved IP addresses.

Project · Name · IP · Type · Location · Status · Used by · Tier · Purpose · Created · Cost/month

#### gImage — Images

Custom images and machine images (vCD).

Project · Name · Type · Family · Status · Deprecation · Disk size (GB) · Stored (GB) · Created · Age (days) · Source · Licenses · Location · Architecture · Labels · Cost/month

#### gReservation — Reservations

Zonal capacity reservations.

Project · Reservation · Zone · Machine type · Reserved · In use · Unused (%) · Specific only · GPUs · Local SSD (GB) · Status · Commitment · Shared · Created

#### gCommitment — Commitments

Resource-based committed use discounts (CUD).

Project · Commitment · Region · Plan · Type · Category · Status · Start · End · Days left · vCPU · RAM (GB) · Other resources · Auto-renew

#### gSoleTenant — Sole-tenant

Sole-tenant nodes and hosted VMs (vHost).

Project · Node group · Zone · Node template · Node · Node type · Status · VM count · VMs · CPU overcommit · Maintenance · Autoscaling

#### gQuota — Quotas

Regional Compute Engine quotas with usage > 0.

Project · Region · Metric · Usage · Limit · Usage (%)

#### gTools — OS and agents *(OS Config module)*

Guest OS inventory via OS Config (vTools).

Project · VM · Zone · Operating system · OS family · Version · Kernel · Architecture · Hostname · OS Config agent · Updated

#### gPerf — Performance *(CPU/RAM metrics module)*

CPU and memory (Cloud Monitoring); memory requires the Ops Agent.

Project · VM · Zone · Status · Machine type · vCPU · RAM (GB) · Days · CPU avg (%) · CPU p95 (%) · CPU max (%) · RAM avg (%) · RAM p95 (%) · RAM max (%) · Ops Agent · Samples

Values are calculated from hourly samples over the chosen period. **p95** is the value below which the metric stays 95% of the time: it's a better sizing indicator than the average. **Samples** is the number of hours with data.

#### gRecommend — Recommendations *(Recommendations module)*

Active recommendations from Google Recommender.

Project · Impact · Type · Subtype · Resource · Location · Description · Monthly savings · Currency · Priority · Status · Updated

#### gCost — Cost *(Estimated cost module)*

Estimated monthly cost at list price (no SUD, CUD or other discounts).

Project · Running VMs · Compute+GPU · Licenses · Disks · Snapshots · Images · Unused IPs · Total/month · Currency

The last row, **TOTAL**, adds up all projects.

#### gCostReal — Actual cost *(actual cost with BigQuery)*

Actual net Compute Engine cost from the Cloud Billing export to BigQuery.

Project · Month · Gross cost · Credits (CUD, SUD, discounts) · Net cost · Attributed to VMs and disks · Unattributed (network, IPs, snapshots, commitments…) · List estimate · Net − estimate · Difference (%) · Currency

#### gProject — Projects

Scanned projects (vSource).

Project · Number · Name · Parent · Status · Scan result · VMs · Running VMs · vCPU · RAM (GB) · Disks · Disks (GB) · Labels · Cost/month · OS Login (project) · Project SSH keys · Serial port (project) · Internet-exposed VMs

#### gHealth — Health

Best-practice findings (vHealth). See [Health rules](#9-health-rules-ghealth).

Severity · Category · Rule · Rule ID · Project · Resource · Location · Detail

Findings are sorted by severity: High, Medium and Low.

#### gSecurity — Security

Security posture per VM: Internet exposure, access and identity. One row per VM.

Project · VM · Zone · Status · Managed by · Issues · External IP · Internet exposed · Sensitive ports open · Rules opening them · Service account · Default SA · cloud-platform scope · OS Login · Accepts project SSH keys · SSH keys on the VM · Serial port · IP forwarding · Secure Boot · vTPM · Integrity monitoring · Confidential · Disk encryption · Notes

- **Internet exposed:** the VM has an external IP and a VPC firewall rule lets traffic in from `0.0.0.0/0` (or `::/0`) to a sensitive port: SSH (22), RDP (3389), Telnet, FTP, VNC, SMB, RPC, NetBIOS, Docker (2375), SQL Server, Oracle, MySQL, PostgreSQL, MongoDB, Redis, Elasticsearch or Memcached.
- **How it's evaluated:** the rules of each NIC's VPC (including those in a Shared VPC host project) that apply to the VM by network tag, service account or to all instances are matched. For each port the rule with the highest priority (lowest number) wins and, on a tie, DENY beats ALLOW, as in GCP. **Rules opening them** shows the deciding ALLOW rules; rules from another project appear as `project/rule`.
- **Limits:** hierarchical firewall policies and network firewall policies aren't read, and access through load balancers isn't considered. If the VPC host project wasn't scanned, exposure is left empty and **Notes** says so.
- **OS Login, SSH keys and serial port:** VM metadata is combined with project metadata (the VM's wins). **Accepts project SSH keys** is Yes when the project has keys, the VM doesn't set `block-project-ssh-keys` and OS Login is off. An organization policy that enforces OS Login isn't visible to gceTools.
- **Disk encryption:** Google, CMEK or CSEK; if the VM's disks use several, all are listed.
- **Issues:** counts how many of these checks fail: Internet exposed, default SA, OS Login off, accepts project SSH keys, serial port, IP forwarding, Secure Boot off and vTPM or integrity monitoring off. On managed VMs OS Login and IP forwarding aren't counted, because those services set them on purpose.

gProject adds, per project, **OS Login (project)**, **Project SSH keys**, **Serial port (project)** and **Internet-exposed VMs**.

#### gErrors — Errors

Projects or APIs that could not be read.

Project · Collector · Type · Message

- **Collector:** what was being read (disks, instances, osconfig, monitoring, cloudasset, pricing, billing…).
- **Type:** `permission_denied` (a permission is missing), `api_disabled` (the API isn't enabled), `not_found` or `error` (anything else).

#### gMetaData — Metadata

Scan information (vMetaData): tool and version, identity, authentication method, OAuth scope, optional modules (including fast mode), quota project, cost currency and cost basis, items not priced, actual cost read, disabled health rules, findings hidden by gcetools-ignore labels, start, end and duration, selected projects and projects OK, errors and whether the scan was cancelled.

## 9. Health rules (gHealth)

Each rule has a **stable ID** that doesn't change between versions or languages. It's used to disable the rule, to exempt resources and in the CLI.

| ID | Rule | Category | Severity | Threshold (default) | Exception on |
|---|---|---|---|---|---|
| `orphan-disk` | Orphaned disk | Cost | High | — | disk or project |
| `stopped-vm` | VM stopped for more than N days | Cost | Medium | stoppedDays (30) | VM or project |
| `unused-ip` | Unused reserved external IP | Cost | Medium | — | project |
| `unused-reservation` | Reservation with unused capacity | Cost | Medium | — | project |
| `cud-expiring` | Commitment (CUD) expiring soon without auto-renewal | Cost | Medium | cudDays (60) | project |
| `cpu-low` | Underused VM (low CPU p95) | Cost | Medium | cpuLowPct (10%) | VM or project |
| `old-family` | Old machine family (N1) | Cost | Low | — | VM or project |
| `old-snapshots` | Old manual snapshots | Cost | Low | snapshotDays (90) | project |
| `firewall-open` | Firewall rule open to the internet | Security | High / Medium | — | project |
| `default-sa` | Compute Engine default service account | Security | Medium / High | — | VM or project |
| `public-ip` | VM with a public IP | Security | Medium | — | VM or project |
| `no-secure-boot` | Secure Boot disabled | Security | Low | — | VM or project |
| `internet-exposed` | VM exposed to the Internet on sensitive ports | Security | High | — | VM or project |
| `project-ssh-keys` | VM accepts project-wide SSH keys | Security | Medium | — | VM or project |
| `serial-port` | Serial port access enabled | Security | Medium | — | VM or project |
| `no-os-login` | OS Login disabled | Security | Low | — | VM or project |
| `ip-forward` | IP forwarding enabled | Security | Low | — | VM or project |
| `no-shielded-vm` | vTPM or integrity monitoring disabled | Security | Low | — | VM or project |
| `template-image` | Template with an obsolete, deprecated or old image | Security / Operations | High, Medium or Low | imageDays (180) | template or project |
| `snapshot-schedule` | VM with disks without a snapshot schedule | Data protection | Low | — | VM or project |
| `cpu-high` | Saturated CPU | Performance | Medium | cpuHighPct (90%) | VM or project |
| `mem-high` | Saturated memory | Performance | Medium | memHighPct (90%) | VM or project |
| `boot-pd-standard` | pd-standard boot disk | Performance | Low | — | VM or project |
| `quota` | Regional quota nearly exhausted | Capacity | Medium / High | quotaPct (80%) | project |
| `no-labels` | VM without labels | Governance | Low | — | VM or project |
| `stopped-no-disks` | Stopped VM without disks | Governance | Low | — | VM or project |
| `no-deletion-protection` | Deletion protection disabled | Operations | Low | — | VM or project |
| `no-ops-agent` | No Ops Agent | Operations | Low | — | VM or project |

### 9.1 What each rule detects

- **orphan-disk:** disks with no VM attached. They're still billed. The detail includes size, type, last detach and monthly cost.
- **stopped-vm:** VMs in TERMINATED state for more than N days. The VM isn't billed for compute, but its disks are; the detail shows that cost.
- **unused-ip:** reserved external IPs that nothing uses. They're billed even when unused.
- **unused-reservation:** active reservations with part of their capacity unused. They're billed all the same.
- **cud-expiring:** active commitments that expire in N days or less and don't renew automatically.
- **cpu-low:** running VMs with at least one week of metrics and CPU p95 below the threshold. Candidates for a smaller machine type.
- **old-family:** N1 VMs. The N2, N2D, N4 or C3 families usually offer better price-performance.
- **old-snapshots:** manually created snapshots (not by a snapshot schedule) older than N days. They're grouped **per project** into a single finding, because backup tools create thousands; the details are in gSnapshot.
- **firewall-open:** enabled ingress rules that allow traffic from `0.0.0.0/0` or `::/0`. They're **High** if they open SSH (22), RDP (3389), all protocols or all TCP ports; **Medium** otherwise.
- **default-sa:** VMs with the Compute Engine default service account (`…-compute@developer.gserviceaccount.com`). It's **High** if the VM also has the `cloud-platform` scope (full API access).
- **public-ip:** VMs with an external IP.
- **no-secure-boot:** VMs without Shielded VM Secure Boot.
- **internet-exposed:** VMs with an external IP and a sensitive port (SSH, RDP, databases…) open to the Internet by the VPC firewall, according to the gSecurity analysis. The detail lists the ports, the IP and the rules.
- **project-ssh-keys:** VMs anyone with an SSH key in the project metadata can log in to: OS Login off and no `block-project-ssh-keys`.
- **serial-port:** VMs with `serial-port-enable=TRUE` (on the VM or the project), which allows logging in through the serial console.
- **no-os-login:** VMs without OS Login: SSH access is managed with metadata keys instead of IAM. If an organization policy already enforces OS Login, disable the rule.
- **ip-forward:** VMs with IP forwarding, which only network appliances, NAT or VPN need.
- **no-shielded-vm:** VMs with Shielded VM vTPM or integrity monitoring off.
- **template-image:** templates that create VMs from a custom image in the scan that is **obsolete or deleted** (High: VMs can no longer be created), **deprecated** (Medium) or older than N days (Low: probably unpatched). Public images and image families aren't evaluated.
- **snapshot-schedule:** VMs with disks without a snapshot schedule. One finding per VM, with the list of disks. If backups are made with another tool (Backup and DR, Veeam…), disable the rule or use the label exception.
- **cpu-high / mem-high:** CPU or RAM p95 above the threshold. RAM requires the Ops Agent.
- **boot-pd-standard:** pd-standard boot disks. pd-balanced is faster and only slightly more expensive.
- **quota:** regional quotas above the threshold; **High** from 95%.
- **no-labels:** VMs without labels. Makes cost allocation harder.
- **stopped-no-disks:** stopped VMs without disks. Usually leftovers from migrations.
- **no-deletion-protection:** VMs that can be deleted without protection.
- **no-ops-agent:** running VMs with metrics but without the Ops Agent: there are no memory or disk metrics.

### 9.2 Managed VMs

For VMs from GKE, Dataproc, Dataflow, Batch and MIGs, seven rules that make no sense on disposable machines, or that those services set on purpose, aren't evaluated:

- `stopped-vm`
- `stopped-no-disks`
- `no-deletion-protection`
- `snapshot-schedule`
- `no-ops-agent`
- `no-os-login`
- `ip-forward` (GKE nodes need it)

The rest (public IP, default service account, Secure Boot…) are evaluated.

### 9.3 Disabling rules and exempting resources

- **Disable a rule** for the whole scan: in **Projects → Health rules**, or with `--disable-rules id1,id2` in the CLI.
- **Exempt a resource** without disabling the rule: add a label to the resource (VM, disk or template) or to its project, according to the "Exception on" column:

| Label | Effect |
|---|---|
| `gcetools-ignore=<id>` | Skips that rule |
| `gcetools-ignore-<id>=true` | The same; lets you combine several rules on the same resource |
| `gcetools-ignore=all` | Skips all rules |

Example: the label `gcetools-ignore=snapshot-schedule` on a project suppresses the warnings about disks without a snapshot schedule in that project.

gMetaData shows which rules were disabled and how many findings were hidden by labels.

## 10. Costs

### 10.1 Estimated cost

The **Estimated cost** module downloads the public Compute Engine pricing catalog (Cloud Billing Catalog API). The catalog is cached for 24 hours.

- **Calculation:** on-demand list price × 730 hours per month, in the chosen currency.
- **Running VM:** compute + GPU + paid licenses + disks.
- **Stopped VM:** disks only.
- **Also calculated:** snapshots (per GB stored), images, machine images and unused IPs.
- **Paid licenses included:** Windows Server, SQL Server, RHEL (also for SAP), SLES (also for SAP) and Ubuntu Pro (also FIPS and Minimal Pro). Container-Optimized OS and the free distributions have no license cost.
- **Not included:** sustained use discounts (SUD), CUDs, negotiated prices, network/egress or BYOL/BYOS licenses. It's a reference, not the invoice.
- **Anything without a price** appears in the "Cost notes" column of gInfo and under "Not priced" in gMetaData.

### 10.2 Actual cost (BigQuery)

If your organization exports billing data to BigQuery ([how to set it up](https://cloud.google.com/billing/docs/how-to/export-data-bigquery)), gceTools reads the **actual net cost** of Compute Engine for a month, with CUDs, SUDs, credits and discounts applied:

- **gCostReal:** per project, gross cost, credits, net cost, list estimate and difference.
- **With the detailed export** (`gcp_billing_export_resource_v1_…`) it also separates what is attributed to VMs and disks from what is unattributed (network, IPs, snapshots, commitments, deleted resources), and fills in the "Actual cost/month (BigQuery)" column of gInfo and gDisks.

**This is the only feature that can cost money:** BigQuery charges for data read (1 TiB free per month per billing account). To keep it under control, gceTools:

1. filters by the month's partitions;
2. first runs a free estimate (*dry run*);
3. doesn't run the query if it would read more than the limit (50 GB by default).

gMetaData shows the GB read.

**Permissions:** `roles/bigquery.jobUser` on the table's project and `roles/bigquery.dataViewer` on the dataset.

## 11. Compare inventories

The **Compare** page shows the additions, removals and changes between two inventories. It doesn't need a Google Cloud connection.

1. **Before (A):** click **Choose JSON snapshot** and select the baseline inventory, for example last month's.
2. **After (B):** choose another JSON snapshot or **Use the current inventory** (the open scan).
3. The comparison runs on its own. **Change** replaces either of the two.

You can also compare from **History** without exporting anything.

**Result:**

- **Figures:** added, removed, modified resources, relevant changes and monthly cost difference.
- **Tabs:**
  - **Summary:** added, removed and changed resources by type.
  - **Added / Removed:** new and deleted resources, with a summary of each one.
  - **Changes:** one field per row, with previous and new value.
  - **Metadata:** the compared inventories, the cost difference and what couldn't be compared.
- **Export:** to Excel, CSV or JSON.

**Relevant changes.** These are flagged and listed first:

- resized VMs;
- service account or Shielded VM changes;
- new public IPs;
- firewall rules open to the internet;
- new or existing VMs that become exposed to the Internet, and changes to their open ports (gSecurity);
- VMs that start accepting project SSH keys or the serial port;
- disks that become orphaned;
- new high-severity findings;
- changes to commitments.

**No false positives:**

- **Permissions:** a resource type is only compared in a project if it was read successfully in both scans. If a project failed in one of them, it appears in Metadata as "Not compared" instead of as hundreds of removals.
- **Noise that is ignored:** last start and stop dates, ages, quota usage, metrics, gCPU/gMemory (they repeat gInfo), gCost, gErrors, gMetaData and snapshots created by a snapshot schedule (they rotate daily).
- **Costs:** only compared if both inventories use the same currency.
- **Columns:** only those present in both inventories are compared, so a snapshot from an earlier version doesn't produce false changes because of new columns.

> Compare inventories taken in the same language: translated texts (severity, finding category…) differ between a scan in Spanish and one in English, and would show up as changes.

## 12. History

Every scan in the app is automatically saved on your computer as compressed JSON. The 100 most recent are kept; when a new one is saved, the oldest is deleted. The **Save every scan automatically** checkbox turns this on or off. The screen shows the folder where scans are saved.

- **Trend charts:** VMs, vCPU, cost (actual if available, otherwise estimated) and high-severity findings, with one point per scan. Hover to see the value and the date.
- **Table:** date, profile, projects, VMs, vCPU, cost, findings by severity and status (complete or partial).
- **With one scan selected:** **Open in Inventory** loads it as if you had just scanned it, to view or export it. **Compare with previous** compares it with the previous scan.
- **With two selected:** **Compare** compares them with each other.
- **Delete from history** deletes the selected scans, after confirmation.

When you open an old scan, the headers are shown in the active language.

## 13. Executive report

**Inventory → Export → Executive report (HTML)** generates a self-contained page (no scripts or external resources). It opens in any browser and can be printed to PDF. It's designed for management or for customers:

- inventory totals;
- VMs exposed to the Internet on sensitive ports (scans from 0.8 onwards);
- the month's estimated or actual cost;
- potential savings according to Recommender;
- the top 10 high or medium severity findings;
- cost by project (the 10 largest);
- projects that couldn't be read;
- if there's history, the changes since the previous scan (up to 10 relevant changes).

In the CLI: `gcetools-cli scan --report report.html`.

## 14. About and updates

- **Version and license** of the app.
- **Website:** documentation, release notes and downloads.
- **Buy me a coffee:** gceTools is free; if it saves you time, you can support its development.
- **License GPL-3.0:** link to the full text.
- **Updates:**
  - **Check for updates** looks up the latest GitHub release; if there's a new one, it shows its release notes and the **Download** button.
  - **Check automatically at startup** runs the check when the app opens.
  - The check is anonymous and doesn't send any data about you or your inventory.
- **Cost of using the Google Cloud APIs:** the table in [API usage cost](#18-api-usage-cost).
- **Third-party components:** libraries used and their licenses.

## 15. Language

gceTools is available in Spanish and English. You change the language with the translate icon in the top bar; the first time, the system language is used. The change is immediate: the open inventory is kept.

- **Interface:** all the app's text.
- **Generated data:** tab and column titles, statuses, gHealth rules and details, metadata, report, exports and error messages. They're generated in the active language **at the time of the scan or the export**.
- **Sheet names** (`gInfo`, `gDisks`…), **rule IDs** and Google's technical values (RUNNING, READY…): they don't change, so as not to break scripts that read the files.
- **Old inventories:** when you open one from history or a JSON snapshot, the headers are shown in the active language; row texts (for example, finding details) stay in the language the scan was taken in.
- **CLI:** the `--lang es|en` option or the `GCETOOLS_LANG` variable. If neither is set, the system language is used (`LC_ALL`, `LC_MESSAGES`, `LANG`).

## 16. Command line

`gcetools-cli` does the same as the app without a graphical interface: it's meant for scheduled tasks, servers and pipelines. It ships in every package and works the same on Windows, macOS and Linux. On Linux and macOS the app binary also accepts the commands (`gcetools scan …`); on Windows it doesn't, because the graphical app doesn't write to the console.

```
gcetools-cli [--lang es|en] <command> [options]
```

| Command | Used for |
|---|---|
| `scan` | Scan projects and export the inventory |
| `diff` | Compare two JSON snapshots |
| `profiles` | List the profiles saved in the app |
| `rules` | List the gHealth rules and how to exempt resources |
| `login` | Sign in with Google without gcloud |
| `logout` | Sign out of the Google session stored in the keyring |
| `version` | Show the version and whether a newer one exists |
| `help` | General help (each command accepts `--help`) |

**Global option:** `--lang es|en` sets the language of messages and generated files. The `GCETOOLS_LANG` variable also works; by default the system language is used. It can go in any position.

**Exit codes:** 0 success; 1 error; 2 invalid options; 3 there are relevant changes (only `diff --fail-on-relevant`). Progress messages and errors go to standard error (stderr).

### 16.1 scan

```
gcetools-cli scan [options]
```

**Authentication:**

| Option | Description | Default |
|---|---|---|
| `--auth` | `adc`, `sa`, `impersonate` or `oauth` (after `gcetools-cli login`) | `adc` |
| `--key` | Path to the service account JSON key (`--auth sa`) | — |
| `--impersonate` | Email of the service account to impersonate (`--auth impersonate`) | — |

**Which projects:**

| Option | Description | Default |
|---|---|---|
| `--projects` | Comma-separated project IDs | all visible |
| `--query` | Resource Manager filter, for example `"parent:folders/123"` | — |
| `--discover` | Scans only the projects with Compute resources according to Cloud Asset Inventory (`organizations/N`, `folders/N`) | — |
| `--profile` | Uses a profile saved in the app; explicit options take precedence | — |

**What to read:**

| Option | Description | Default |
|---|---|---|
| `--modules` | Comma-separated optional modules: `osconfig`, `monitoring`, `recommender`, `pricing`; `none` turns them off | all four |
| `--metrics-days` | Days of Cloud Monitoring metrics | 30 |
| `--currency` | Estimated cost currency (USD, EUR, CLP…) | USD |
| `--fast` | Fast mode with Cloud Asset Inventory; with `--discover`, a single read for the whole scope | off |
| `--billing-table` | BigQuery billing export table (`project.dataset.table`) for actual cost | — |
| `--billing-month` | Billing month `YYYYMM` | the previous month |
| `--billing-max-gb` | Skip the BigQuery query if it would read more GB than this | 50 |
| `--disable-rules` | Comma-separated gHealth rules to disable | — |

**Output:**

| Option | Description | Default |
|---|---|---|
| `--out` | Output file: `.xlsx`, `.zip` or `.csv` (ZIP of CSVs), `.json` | `gcetools-YYYYMMDD-HHMM.xlsx` |
| `--format` | `xlsx`, `csv` or `json`, if you don't want it inferred from the extension | from the extension |
| `--csv-sep` | CSV separator (use `;` for Excel with Spanish regional settings) | `,` |
| `--report` | Also generates the HTML executive report at this path | — |
| `--save-history` | Also saves the result to the local history (the same one the app uses) | off |

**Performance:**

| Option | Description | Default |
|---|---|---|
| `--concurrency` | Projects in parallel | 8 |
| `--rps` | Maximum requests per second | 20 |

**Examples:**

```bash
# Everything visible with ADC, to Excel
gcetools-cli scan --out inventory.xlsx

# A folder with a service account, CSV for Excel with Spanish regional settings
gcetools-cli scan --auth sa --key ./sa.json --query "parent:folders/123456" --out inv.zip --csv-sep ";"

# Two projects, impersonating a service account
gcetools-cli scan --auth impersonate --impersonate inventory@prj.iam.gserviceaccount.com --projects a,b

# Whole organization, only projects with Compute, in fast mode
gcetools-cli scan --discover organizations/123456 --fast --out inventory.xlsx

# With an app profile, saving to history and generating the report
gcetools-cli scan --profile production --save-history --report report.html --out inventory.xlsx

# October actual cost from the billing export
gcetools-cli scan --billing-table finops-prj.billing.gcp_billing_export_resource_v1_XXXXXX --billing-month 202610

# Fast and without optional modules, in English
gcetools-cli --lang en scan --modules none --out inventory.json
```

### 16.2 diff

```
gcetools-cli diff [options] before.json after.json
```

Compares two JSON snapshots with the same rules as the Compare page.

| Option | Description | Default |
|---|---|---|
| `--out` | Output file (`.xlsx`, `.zip`/`.csv`, `.json`) | `gcetools-diff-YYYYMMDD-HHMM.xlsx` |
| `--format` | `xlsx`, `csv` or `json` | from the extension |
| `--csv-sep` | CSV separator | `,` |
| `--fail-on-relevant` | Exits with code 3 if there are relevant changes | off |

Besides the file, it prints the summary and up to 20 relevant changes to the console, which is handy in cron emails.

```bash
gcetools-cli diff --out changes.xlsx monday.json friday.json
gcetools-cli diff --fail-on-relevant base.json today.json   # for alerts or pipelines
```

### 16.3 login and logout

```bash
gcetools-cli login                                   # opens the browser; the session stays in the keyring
gcetools-cli login --client client_secret_XXXX.json  # with your organization's own OAuth client
gcetools-cli scan --auth oauth --out inventory.xlsx
gcetools-cli logout                                  # removes the session from the keyring
```

The session is the same one the app uses: if you already signed in in the app, `--auth oauth` works without `login`.

### 16.4 profiles, rules and version

```bash
gcetools-cli profiles   # profiles saved in the app and path of profiles.json
gcetools-cli rules      # the 28 rules with their ID, severity, category and threshold
gcetools-cli version    # installed version and whether a newer one exists
```

### 16.5 Scheduled tasks

**Linux or macOS (cron), every Monday at 7:00:**

```bash
0 7 * * 1 /usr/bin/gcetools-cli scan --profile production --save-history --report "$HOME/reports/report-$(date +\%F).html" --out "$HOME/reports/inventory-$(date +\%F).xlsx"
```

**Windows (Task Scheduler):** create a task with the *Start a program* action:

- **Program/script:** `C:\Program Files\gceTools\gcetools-cli.exe` (or the folder where you installed it).
- **Add arguments:** `scan --auth sa --key C:\gcetools\sa.json --profile production --out C:\reports\inventory.xlsx`

**Change alerts:** save a JSON snapshot every day and compare it with the previous one. `--fail-on-relevant` returns 3 if there are relevant changes, which lets you trigger an alert from the script or the pipeline:

```bash
gcetools-cli scan --auth sa --key sa.json --out today.json
gcetools-cli diff --fail-on-relevant --out changes.xlsx yesterday.json today.json || echo "There are relevant changes"
```

In scheduled tasks use a service account (`--auth sa` or `--auth impersonate`): user sessions can expire or require reauthentication.

## 17. IAM permissions and APIs

### 17.1 Roles by feature

| Feature | Required role | Where |
|---|---|---|
| Basic inventory | `roles/compute.viewer` | Organization, folder or project |
| List projects and their hierarchy | `roles/browser` | Organization or folder |
| OS and agents (gTools) | `roles/osconfig.inventoryViewer` | Project |
| Metrics (gPerf) | `roles/monitoring.viewer` | Project |
| Recommendations (gRecommend) | `roles/recommender.computeViewer` | Project |
| Detect projects and fast mode | `roles/cloudasset.viewer` | Organization, folder or project |
| Actual cost | `roles/bigquery.jobUser` and `roles/bigquery.dataViewer` | Table's project and dataset |
| Impersonation | `roles/iam.serviceAccountTokenCreator` | On the service account |

**gSecurity and Shared VPC:** to tell whether a VM in a service project is exposed, gceTools needs the host project's firewall rules, so the host project must be in the scan and the identity needs `roles/compute.viewer` on it. Project metadata (OS Login, SSH keys, serial port) is also read with `roles/compute.viewer`.

For a global view, grant the roles at the organization or folder level. Without `resourcemanager.projects.list` you can add projects manually by ID.

### 17.2 Quota project

Some APIs (Recommender, Cloud Asset Inventory, Cloud Billing Catalog) charge quota to a "consumer" project instead of the project being read:

- **gcloud credentials:** the ADC `quota_project_id` (`gcloud auth application-default set-quota-project <project>`).
- **Service account:** its own project.
- **Sign in with Google:** the OAuth client's project (gceTools' project, or your organization's if you use your own client).

That project must have the corresponding APIs enabled.

### 17.3 APIs to enable

In the **projects being scanned**:

- **Compute Engine API:** required.
- **OS Config API:** for gTools.
- **Cloud Monitoring API:** usually enabled already.

In the **quota project**:

- **Recommender API**, **Cloud Asset API** and **Cloud Billing API** (pricing catalog), depending on the modules you use.
- **BigQuery API:** in the project where the billing query runs.

If you use **your own OAuth client**, its project must have every API gceTools uses enabled:

```bash
gcloud services enable --project <client-project> cloudresourcemanager.googleapis.com compute.googleapis.com monitoring.googleapis.com osconfig.googleapis.com recommender.googleapis.com cloudasset.googleapis.com cloudbilling.googleapis.com bigquery.googleapis.com
```

## 18. API usage cost

gceTools only reads. Almost all APIs are free.

| API | Used for | Cost |
|---|---|---|
| Compute Engine | Inventory of VMs, disks, network, snapshots… | Free (per-minute quotas only) |
| Resource Manager | List projects | Free |
| Cloud Billing Catalog | Prices for estimated cost (24 h cache) | Free |
| Recommender | Recommendations | Free |
| Cloud Asset Inventory | Detect projects and fast mode | Free (with per-minute quotas) |
| Cloud Monitoring | CPU and RAM metrics | USD 0.50 per million series; the first million per month per billing account is free |
| OS Config | OS and agents | Reading is free (VM Manager bills per VM with an active agent, not for gceTools) |
| BigQuery | Actual cost (optional, off by default) | Per data read (1 TiB free per month); gceTools estimates first and respects a GB limit |
| GitHub | Check for updates | Free |

For reference, a scan requests about 2 Monitoring series per VM: with 100 VMs it would take about 5,000 scans a month to exceed the free tier.

## 19. Local files and data

| What | Linux | Windows | macOS |
|---|---|---|---|
| History | `~/.local/share/gcetools/history` | `%LocalAppData%\gcetools\history` | `~/Library/Application Support/gcetools/history` |
| Profiles (`profiles.json`, permissions 0600) | `~/.config/gcetools` | `%AppData%\gcetools` | `~/Library/Application Support/gcetools` |
| Pricing catalog cache | `~/.cache/gcetools` | `%LocalAppData%\gcetools` | `~/Library/Caches/gcetools` |
| Google session | Secret Service (service `gcetools`) | Credential Manager (`gcetools`) | Keychain (`gcetools`) |

- **History:** `index.json` plus one `<id>.json.gz` file per scan.
- **Interface preferences** (language, theme, save history, hide managed VMs, check for updates): stored in the app's local storage.
- **Service account key loaded in the app:** in memory only; never written to disk.

To start from scratch, delete those folders and sign out of Google from the app (or remove the `gcetools` entry from the keyring).

## 20. Privacy and security

- **No servers of its own:** gceTools doesn't send data to its author or to third parties. It only communicates with the Google APIs and, to check for updates, with the public GitHub API, anonymously and with an option to turn it off.
- **No tracking:** it uses no analytics, telemetry, cookies or advertising.
- **Read-only:** access to Google Cloud is read-only.
- **Credentials:** session tokens go to the system keyring; service account keys are only kept in memory; profiles don't store secrets.
- **Open source:** GPL-3.0, auditable on [GitHub](https://github.com/jordilopezr/gcetools).

Full policy: [PRIVACY.en.md](../PRIVACY.en.md).

## 21. Troubleshooting

| Message or symptom | Cause | Solution |
|---|---|---|
| "The gcloud session has expired or your organization requires you to reauthenticate" | The ADC token expired or your organization requires periodic reauthentication | `gcloud auth application-default login` and connect again |
| "The Google session has expired or access was revoked" | Access was revoked or the session expired (while the app is in testing mode, every 7 days) | **Sign in with Google** again |
| Google shows "Access blocked" or "Google hasn't verified this app" | The app isn't verified yet and your account isn't a test user, or your organization blocks external apps | Use another connection method, ask your administrator to allow gceTools or use your organization's OAuth client |
| "Keyring unavailable" | There's no keyring service (Linux without GNOME Keyring or KWallet) | Install and unlock one, or use another method |
| No projects appear | The identity doesn't have `resourcemanager.projects.list` | Add the projects by ID or request `roles/browser` |
| "could not list projects: … API has not been used in project N" | The Cloud Resource Manager API isn't enabled in the quota project (or in the OAuth client's project) | Enable it in that project and wait a few minutes |
| Project "Compute API disabled" | The project doesn't use Compute Engine | Normal; to avoid scanning it, use **Detect projects with Compute** |
| Project "Permission denied" | `roles/compute.viewer` is missing | Grant the role on the project or on a parent folder |
| Project "OK (without osconfig)", "OK (without recommender)"… | The module isn't available in that project (API disabled or a permission missing) | The exact reason is in gErrors |
| gRecommend empty with user credentials | There's no quota project, or it doesn't have the Recommender API | `gcloud auth application-default set-quota-project <project>` and enable the API in it |
| Cloud Asset Inventory error when detecting or in fast mode | `roles/cloudasset.viewer` is missing on the scope, or the API in the quota project | Grant the role or enable the Cloud Asset API; fast mode falls back to the Compute API automatically |
| gPerf without RAM data | The VMs don't have the Ops Agent | Install the Ops Agent; the `no-ops-agent` rule flags them |
| No cost columns | The Estimated cost module is turned off or the catalog couldn't be downloaded | Turn it on; the error, if any, is in gErrors (`pricing` collector) |
| "the query would read X GB, more than the N GB limit" | The actual cost query exceeds the configured limit | Raise the limit, check the chosen month or use another table |
| The scan is slow | Many projects or Compute API quotas | Use **Detect projects with Compute**, **fast mode**, more parallelism or fewer modules |
| Windows: "Unknown publisher" | The executables aren't signed yet | **More info → Run anyway** |
| macOS: "can't be opened because it is from an unidentified developer" | Gatekeeper blocks unsigned apps | Right-click → Open, or `xattr -dr com.apple.quarantine /Applications/gceTools.app` |
| Linux: the AppImage doesn't start | GTK 3 or WebKitGTK 4.1 is missing | Install them (`libwebkit2gtk-4.1-0`) or use the `.deb`/`.rpm` |
| gSecurity: **Internet exposed** empty and "No firewall data for VPC …" in Notes | The VM's VPC is in another project (Shared VPC) that wasn't scanned or couldn't be read | Add the host project to the scan with `roles/compute.viewer` |
| gSecurity flags OS Login as off although the organization enforces it | The organization policy isn't visible to gceTools | Turn off the `no-os-login` rule in **Projects → Health rules** (or `--disable-rules no-os-login`) |
| The CSV opens in a single column in Excel | Excel with Spanish regional settings expects `;` | Export as **CSV (semicolon)** or use `--csv-sep ";"` |

If the problem persists, open an issue on [GitHub](https://github.com/jordilopezr/gcetools/issues) with the version (`gcetools-cli version`) and the contents of gErrors, without sensitive data.

## 22. Frequently asked questions

**Can gceTools change anything in my Google Cloud?**
No. It only requests the read-only scope and only runs queries.

**Does it cost money to use?**
The app is free. The APIs it uses are free except Cloud Monitoring, whose free tier is generous, and BigQuery if you turn on actual cost. See [API usage cost](#18-api-usage-cost).

**Where is my data?**
On your computer. See [Local files and data](#19-local-files-and-data).

**Why doesn't the estimated cost match my invoice?**
Because it's list price, without sustained use discounts, CUDs or negotiated prices, and without network. For the actual cost, use the Cloud Billing export to BigQuery.

**Can I scan several organizations?**
Yes, if your identity has access: the project list includes everything it can see. To scan a specific organization, use the filter or the detection scope.

**How often should I scan?**
For tracking, once a week or once a month, with history turned on. To detect changes, daily with the CLI and `diff --fail-on-relevant`.

**Does fast mode give the same data?**
Yes, for the resource types it covers, except that Cloud Asset Inventory can take a few minutes to reflect the most recent changes. To check, scan in both modes and compare the results.

**Does gSecurity replace Security Command Center?**
No. It's a quick review of each VM's configuration based on the inventory: it doesn't read hierarchical firewall policies or network firewall policies, load balancers, organization policies or operating system vulnerabilities. It's for finding the obvious (SSH or RDP open to the Internet, project SSH keys, serial port) and tracking it over time with history and comparison.

**Can I add my own rules or tabs?**
The code is open source: the README explains how to add a tab. Rule proposals are welcome on GitHub.

## 23. RVTools equivalents

| RVTools | gceTools |
|---|---|
| vInfo | gInfo |
| vCPU / vMemory | gCPU / gMemory |
| vDisk | gDisk |
| vNetwork | gNetwork |
| vDatastore | gDisks |
| vSnapshot | gSnapshot |
| vLicense | gLicense |
| vCluster | gMIG / gTemplate |
| vSwitch / dvPort | gVPC / gSubnet (plus gFirewall and gAddress) |
| vCD | gImage |
| vHost | gSoleTenant |
| vTools | gTools |
| vSource | gProject |
| vHealth | gHealth |
| — | gSecurity (security posture per VM) |
| vMetaData | gMetaData |
| — | gReservation, gCommitment, gQuota, gPerf, gRecommend, gCost, gCostReal, gErrors |
