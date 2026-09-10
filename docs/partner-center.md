# AI Gauge — Microsoft Partner Center Listing Content

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
  1. Automatic quota and reset-time monitoring
  2. Multiple AI usage providers with visibility and ordering controls
  3. Configurable background refresh intervals
  4. Windows system tray access with light, dark, and system themes
  5. Built-in sample preview - explore the app before setting up any provider
- **Screenshot captions** (Desktop, 2 images):
  1. AI Gauge usage dashboard in dark mode.
  2. AI Gauge usage dashboard in light mode.
- **Keywords**: AI usage monitor, system tray, quota tracker, usage dashboard, background usage refresher
- **Copyright/trademark info**: © 2026 jmnote. Licensed under the Apache License 2.0.
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
> AI Gauge is an independently developed Windows system tray utility that displays remaining quotas and reset times for supported AI development tools: OpenAI Codex, Anthropic Claude Code, and Google Antigravity.
>
> The supported third-party tools are not required for certification testing. AI Gauge includes a built-in sample-data preview so its primary functionality can be evaluated on a clean device without installing third-party tools, signing in, or providing account credentials.
>
> Testing instructions:
>
> 1. Launch AI Gauge. The first-run screen, "Quota usage and reset times", explains what the app displays and what live monitoring requires.
> 2. On a clean device, confirm that Codex, Claude Code, and Antigravity each show a clear "Not installed" state with setup guidance, rather than raw file paths, command errors, or an indefinite loading state.
> 3. Select **Preview with sample data**.
> 4. Confirm that the **Sample data** badge remains visible throughout the preview.
> 5. Verify the quota gauges, remaining percentages, reset times, and last-updated information.
> 6. Select a provider's refresh control and confirm that the sample data refreshes without requesting credentials or network access.
> 7. Open Settings and verify the warning and critical thresholds and the Light, Dark, and System themes.
> 8. Select **Back to setup** and confirm that the first-run screen returns without a crash or raw system error.
> 9. Verify the Always-on-top control and the system tray minimize and restore behavior.
>
> The preview uses data bundled with the application and requires no network access, third-party tool installation, login, or credentials.
>
> On the first-run screen, AI Gauge performs only local readiness checks. A provider that is ready for verification offers a **Check connection** button, and no usage request is made for that provider until the button is selected. A successful connection enables the provider and starts periodic usage requests at the configured refresh interval. Enabling a provider in Settings also starts periodic usage requests. Before a usage request is made, each provider reports a locally determined readiness state, such as "Not installed", "Check connection", or "Login required".
>
> The first-run screen, sample-data preview, and explicit provider states allow certification testers to evaluate the app's primary functionality on a clean device without installing or signing in to any supported AI provider.
>
- **Credentials for certification**: Leave empty. The sample preview must be testable without credentials.
- **Pre-entry verification**:
  1. Confirm the sample preview opens from the first-run screen in the packaged app, with no command-line arguments.
  2. Confirm that every button name in the notes above matches the uploaded MSIX exactly: **Preview with sample data**, **Back to setup**, **Check connection**.
  3. Confirm that entering and leaving the preview leaves the saved provider settings unchanged.
  4. Test from a clean Windows account with no Codex, Claude Code, or Antigravity installation or login state, and confirm no raw error text or failure counter appears.
  5. Enter and save the notes only after the updated package has been uploaded to the draft submission.
