# gceTools privacy policy

*Last updated: November 21, 2026* · [Español](PRIVACY.md)

gceTools is an open source desktop application (GPL-3.0 license) that builds an inventory of the Google Compute Engine resources the user's account can access. This policy explains what data it handles and what it does with it.

## Summary

- gceTools **has no servers of its own** and **sends no data to its author or to third parties**. It only talks to Google APIs and, to check for new versions, to GitHub.
- The whole inventory is processed and stored **on the user's computer**.
- Access to Google Cloud is **read-only**.

## Google data it accesses

When you sign in with Google, gceTools requests these permissions:

| Permission | Why |
|---|---|
| `https://www.googleapis.com/auth/cloud-platform.read-only` | To read, without modifying, the Google Cloud resources the account already has access to: projects and, if the user configures it, the billing export in BigQuery |
| `https://www.googleapis.com/auth/compute.readonly` | To read Compute Engine virtual machines, disks, networks, snapshots, images and quotas |
| `https://www.googleapis.com/auth/monitoring.read` | To read VM CPU and memory metrics |
| `https://www.googleapis.com/auth/cloud-billing.readonly` | To read the public price catalog to estimate costs |
| `openid`, `email` | To show in the app which account is signed in |

gceTools never creates, modifies or deletes Google Cloud resources.

## What it stores and where

Everything stays on the user's computer:

- **Sign-in token:** the Google refresh token is stored in the operating system keyring (Credential Manager on Windows, Keychain on macOS, Secret Service on Linux). "Sign out" deletes it.
- **Inventories:** the files the user exports (XLSX, CSV, JSON, HTML) and, when enabled, the scan history in the user data folder.
- **Preferences and profiles:** in the user config folder. They contain no secrets.

## Network connections

- **Google APIs** (`*.googleapis.com`, `accounts.google.com`, `oauth2.googleapis.com`): to sign in and read the inventory.
- **GitHub** (`api.github.com`): an anonymous request to check whether a new version exists, which can be turned off in "About". It includes no user or inventory data.

No analytics, telemetry, cookies or advertising are used.

## Use of Google data

gceTools' use of information received from Google APIs adheres to the [Google API Services User Data Policy](https://developers.google.com/terms/api-services-user-data-policy), including the Limited Use requirements. The data is used only to show and export the inventory to the user themselves. It is not transferred to third parties, not used for advertising and not used to train artificial intelligence models.

## Revoking access

At any time, from gceTools ("Sign out") or from [your Google Account permissions page](https://myaccount.google.com/permissions).

## Contact

For privacy questions, open an issue at https://github.com/jordilopezr/gcetools/issues or get in touch through https://gcetools.jordilopezr.com.
