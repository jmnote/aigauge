# AI Gauge — Microsoft Partner Center Listing Content

This document records only the content directly entered/selected by the developer in the [Microsoft Partner Center](https://partner.microsoft.com/dashboard) submission form for **AI Gauge** (system-generated identifiers, submission status, and validation results are intentionally omitted). Recorded on: 2026-09-02. Draft fields reviewed on: 2026-09-04.

## Product

- **Product name**: AI Gauge
- **Product type**: MSIX or PWA app

## Properties

- **Category / Subcategory**: Utilities + tools (no subcategory selected)
- **Uses personal information?**: Yes, my product uses personal information
- **Privacy policy**: See [`docs/privacy-policy.md`](./privacy-policy.md), the single source of truth for the policy text.
  - **Partner Center URL**: https://github.com/jmnote/aigauge/blob/main/docs/privacy-policy.md
- **Product declarations** checked:
  - Customers can install this product to alternate drives or removable storage.
  - Windows can include this product's data in automatic backups to OneDrive.
  - Customers can use Windows 10/11 features to record and broadcast clips of this product.
- **System requirements**: None specified
- **Support info**:
  - Website: https://github.com/jmnote/aigauge
  - Support contact info: https://github.com/jmnote/aigauge/issues
  - Phone / address: not provided

## Pricing and availability

- **Markets**: All worldwide markets (240 markets)
- **Base price**: KRW 0 (Free)
- **Audience**: Public audience
- **Discoverability**: Available and discoverable in the Microsoft Store
- **Schedule**: Release as soon as possible / Stop acquisition: never
- **Free trial**: Not configured
- **Sale pricing**: Not configured
- **Organizational licensing**: Volume acquisition by organizations not allowed

## Store listing (English — United States)

- **Description**:
  > AI Gauge is a lightweight Windows system tray widget for monitoring usage across OpenAI Codex, Anthropic Claude Code, and Google Antigravity. It shows remaining quota percentages, reset times, connection status, and the last successful update in a compact desktop window. You can show, hide, and reorder providers; configure refresh intervals and warning and critical thresholds; keep the widget above other windows; and choose a light, dark, or system theme. Live monitoring uses what is already on your PC: Codex and Claude Code are read from their existing local login, and Antigravity uses the locally installed agy command-line tool, so those providers need the corresponding tool set up and logged in. A built-in sample preview lets you see how the app works before configuring anything - the preview shows bundled example data, not usage from any account. AI Gauge runs locally, does not store a separate copy of your credentials, and does not retain usage data on a remote server.
- **Short description**: Monitor AI service usage, quotas, reset times, and connection status from your Windows system tray.
- **Product features**:
  1. Real-time quota and reset countdown monitoring
  2. Multiple AI usage providers with visibility and ordering controls
  3. Configurable background refresh intervals
  4. Windows system tray access with light, dark, and system themes
  5. Built-in sample preview - explore the app before setting up any provider
- **Screenshot captions** (Desktop, 2 images):
  1. AI Gauge usage dashboard in dark mode.
  2. AI Gauge usage dashboard in light mode.
- **Keywords**: AI usage monitor, system tray, quota tracker, usage dashboard, background usage refresher
- **Copyright/trademark info**: Apache License 2.0
- **Developed by**: jmnote

## Submission options

- **Publishing**: Publish this submission as soon as it passes certification (default option selected)
- **Restricted capability justification — runFullTrust** (required text, entered by developer):
  > AI Gauge is a Wails-based Windows desktop application. It requires runFullTrust because it runs as a native Win32 process, provides a system tray UI, and invokes the locally installed agy CLI to retrieve usage data. The capability is required for the application's core desktop functionality.
- **Notes for certification**: Managed on **Supplemental info → Additional Testing Information**, rather than directly on the Submission Options page.
  - **Description**: Not provided
  - **Credentials**: None

### Certification notes for the sample-preview release

> **Status:** Draft text only. Do not enter this text in Partner Center until the first-run screen and the sample-data preview described below are present in the uploaded MSIX package. Version `v0.5.0` has neither: it exposes fixtures only through the developer-only `--fixtures` flag.

Enter the following text in **Supplemental info → Additional Testing Information → Notes for Certification → Description** after uploading the package:

> Dear Microsoft Certification Team,
>
> Thank you for reviewing AI Gauge.
>
> AI Gauge is an independently developed Windows system tray utility that displays remaining quotas and reset times for locally configured AI development tools: OpenAI Codex, Anthropic Claude Code, and Google Antigravity.
>
> A certification device is not expected to have those third-party tools installed or signed in. This submission includes a built-in sample-data preview so the app's primary functionality can be evaluated on a clean device without installing anything or supplying account credentials.
>
> Testing instructions:
>
> 1. Launch AI Gauge. The first-run screen, "Quota usage and reset times", explains what the app displays and what live monitoring requires.
> 2. On a clean device, confirm that Codex, Claude, and Antigravity each show a supported "Not installed" state with setup guidance - not raw file paths, command errors, or an indefinite loading state.
> 3. Select **Preview with sample data**.
> 4. Confirm that a **Sample data** badge is shown and stays visible for the whole preview.
> 5. Verify the quota gauges, remaining percentages, reset countdowns, and last-updated information.
> 6. Select a provider's refresh control and confirm the preview refreshes without requesting credentials or network access.
> 7. Open Settings and verify the warning and critical thresholds and the Light, Dark, and System themes.
> 8. Select **Back to setup** and confirm the first-run screen returns without a crash or a raw system error.
> 9. Verify the Always-on-top control and the system tray minimize and restore behavior.
>
> The preview uses data bundled inside the application and requires no network access, third-party tool installation, login, or credentials. No credentials are required for certification testing.
>
> AI Gauge contacts a provider service only when the user asks it to. On the first-run screen, a provider that is ready to be verified offers a **Check connection** button, and no network request is made for that provider until it is selected. Until then each provider reports only what was determined locally: "Not installed", "Check connection", or "Login required".
>
> The first-run screen, the sample preview, and this provider-state handling were added in response to the previous 10.1.2.10 Functionality result, in which AI detection could not be evaluated on a clean certification device.
>
> Product ID: 9MT65KM56P99
>
> For questions, please contact the publisher through the support information in the Store listing. Thank you.

- **Credentials for certification**: Leave empty. The sample preview must be testable without credentials.
- **Pre-entry verification**:
  1. Confirm the sample preview opens from the first-run screen in the packaged app, with no command-line arguments.
  2. Confirm that every button name in the notes above matches the uploaded MSIX exactly: **Preview with sample data**, **Back to setup**, **Check connection**.
  3. Confirm that entering and leaving the preview leaves the saved provider settings unchanged.
  4. Test from a clean Windows account with no Codex, Claude Code, or Antigravity installation or login state, and confirm no raw error text or failure counter appears.
  5. Enter and save the notes only after the updated package has been uploaded to the draft submission.
