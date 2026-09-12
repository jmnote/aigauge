# AI Gauge — Microsoft Partner Center submission

## Product

- **Product name**: AI Gauge
- **Product type**: MSIX or PWA app

## Pricing and availability

- **Markets**: All worldwide markets (240 markets)
- **Visibility**:
  - **Audience**: Public audience
  - **Discoverability**: Make this product available and discoverable in the Microsoft Store.
- **Schedule**: Release: as soon as possible / Stop acquisition: never
- **Pricing**:
  - **Base price**: KRW 0 (Free)
- **Free trial**: Not configured
- **Sale pricing**: Not configured
- **Organizational licensing**: Volume acquisition by organizations not allowed

## Properties

- **Category**: Utilities + tools
- **Subcategory**: None selected
- **Uses personal information?**: Yes, my product uses personal information
- **Privacy policy URL**: https://github.com/jmnote/aigauge/blob/main/docs/privacy-policy.md
- **Support info**:
  - **Website**: https://github.com/jmnote/aigauge
  - **Support contact info**: https://github.com/jmnote/aigauge/issues
  - **Phone number**: Not provided
  - **Address**: Not provided
- **Display mode**: PC and HoloLens not selected
- **Product declarations**:
  - Customers can install this product to alternate drives or removable storage.
  - Customers can use Windows 10/11 features to record and broadcast clips of this product.
- **System requirements**: None specified

## Age ratings

- **Rating questionnaire**: IARC questionnaire completed

## Packages

- **Device family availability**: Windows 10/11 Desktop
- **Package**: `aigauge_<version>_x64.msix` (for example, `aigauge_0.6.2.0_x64.msix`)
- **Version**: Four-part MSIX version derived from the release tag (for example, `v0.6.2` → `0.6.2.0`)
- **Architecture**: X64
- **Device families**: Windows.Desktop min version 10.0.17763.0

## Store listings

### English (United States)

- **Product name**: AI Gauge
- **Description**:
  > AI Gauge is a lightweight Windows system tray widget for monitoring usage across OpenAI Codex, Anthropic Claude Code, and Google Antigravity. Live monitoring requires an existing signed-in Codex or Claude Code session, or the locally installed and signed-in agy command-line tool for Antigravity. A built-in sample preview is available without installing third-party tools or signing in to a provider. AI Gauge shows remaining quota percentages, reset times, connection status, and the last successful update in a compact desktop window. You can show, hide, and reorder providers; configure refresh intervals and warning and critical thresholds; keep the widget above other windows; toggle visibility with a global hotkey; and choose a light, dark, or system theme. The sample preview shows bundled example data, not usage from any account. AI Gauge runs locally, does not store a separate copy of your credentials, and does not retain usage data on a remote server.
- **What's new in this version**: Managed in [`submission-overrides.yaml`](submission-overrides.yaml)
- **Product features**: Managed in [`submission-overrides.yaml`](submission-overrides.yaml)
- **Screenshots**: 2 Desktop images
  1. AI Gauge usage dashboard in dark mode.
  2. AI Gauge usage dashboard in light mode.
- **Supplemental fields**:
  - **Short description**: Monitor AI service usage, quotas, reset times, and connection status from your Windows system tray.
  - **Keywords**: AI usage monitor, system tray, quota tracker, usage dashboard, background usage refresher
  - **Copyright and trademark info**: © 2026 jmnote. Licensed under the Apache License 2.0.
  - **Developed by**: jmnote

## Submission options

- **Publishing**: Publish this submission as soon as it passes certification
- **Restricted capability justification — runFullTrust**:
  > AI Gauge is a Wails-based Windows desktop application. It requires runFullTrust because it runs as a native Win32 process, provides a system tray UI, and invokes the locally installed agy CLI to retrieve usage data. The capability is required for the application's core desktop functionality.

## Supplemental info

### Additional Testing Info

#### Notes for Certification

Managed in [`submission-overrides.yaml`](submission-overrides.yaml).

#### Credentials

- None
