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
- **Package**: `aigauge_0.6.1.0_x64.msix`
- **Version**: v0.6.1.0
- **Architecture**: X64
- **Device families**: Windows.Desktop min version 10.0.17763.0

## Store listings

### English (United States)

- **Product name**: AI Gauge
- **Description**:
  > AI Gauge is a lightweight Windows system tray widget for monitoring usage across OpenAI Codex, Anthropic Claude Code, and Google Antigravity. It shows remaining quota percentages, reset times, connection status, and the last successful update in a compact desktop window. You can show, hide, and reorder providers; configure refresh intervals and warning and critical thresholds; keep the widget above other windows; and choose a light, dark, or system theme. Live monitoring uses what is already on your PC: Codex and Claude Code are read from their existing local login, and Antigravity uses the locally installed agy command-line tool, so those providers need the corresponding tool set up and logged in. A built-in sample preview lets you see how the app works before configuring anything - the preview shows bundled example data, not usage from any account. AI Gauge runs locally, does not store a separate copy of your credentials, and does not retain usage data on a remote server.
- **What's new in this version**: Not provided
- **Product features**:
  1. Automatic quota and reset-time monitoring
  2. Multiple AI usage providers with visibility and ordering controls
  3. Configurable background refresh intervals
  4. Windows system tray access with light, dark, and system themes
  5. Built-in sample preview - explore the app before setting up any provider
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

#### Notes for Certification — Description

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
> 1. Launch AI Gauge. The first-run screen, "Quota usage and reset times", explains what the app displays and what is required for live monitoring.
> 2. On a clean device, confirm that Codex, Claude Code, and Antigravity each show a clear "Not installed" state with setup guidance, rather than raw file paths, command errors, or an indefinite loading state.
> 3. Select Preview with sample data.
> 4. Confirm that the Sample data badge remains visible throughout the preview.
> 5. Verify the quota gauges, remaining percentages, reset times, and last-updated information.
> 6. Select a provider's refresh control and confirm that the sample data refreshes without requesting credentials or network access.
> 7. Open Settings and verify the warning and critical thresholds and the Light, Dark, and System themes.
> 8. Select Back to setup and confirm that the first-run screen returns without a crash or raw system error.
> 9. Verify the Always-on-top control and the system tray minimize and restore behavior.
>
> The preview uses data bundled with the application and requires no network access, third-party tool installation, login, or credentials.
>
> On the first-run screen, AI Gauge performs only local readiness checks. A provider that is ready for verification offers a Check connection button, and no usage request is made for that provider until the button is selected. A successful connection enables the provider and starts periodic usage requests at the configured refresh interval. Enabling a provider in Settings also starts periodic usage requests. Before a usage request is made, each provider reports a locally determined readiness state, such as "Not installed", "Check connection", or "Login required".
>
> The first-run screen, sample-data preview, and explicit provider states allow certification testers to evaluate the app's primary functionality on a clean device without installing or signing in to any supported AI provider.

#### Credentials

- None
