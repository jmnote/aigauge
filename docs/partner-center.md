# AI Gauge — Microsoft Partner Center Listing Content

This document records only the content directly entered/selected by the developer in the [Microsoft Partner Center](https://partner.microsoft.com/dashboard) submission form for **AI Gauge** (system-generated identifiers, submission status, and validation results are intentionally omitted). Recorded on: 2026-09-02.

## Product

- **Product name**: AI Gauge
- **Product type**: MSIX or PWA app

## Properties

- **Category / Subcategory**: Utilities + tools (no subcategory selected)
- **Uses personal information?**: Yes, my product uses personal information
- **Privacy policy** (entered as text, not URL):
  > AI Gauge Privacy Policy
  >
  > AI Gauge is a standalone Windows desktop application that displays usage information for OpenAI Codex, Anthropic Claude Code, and Google Antigravity.
  >
  > **Data access and use:** AI Gauge reads existing local Codex and Claude Code login sessions and invokes the locally installed agy command-line tool when Antigravity monitoring is enabled. It uses the returned information only to display quotas, reset times, and connection status locally. AI Gauge does not operate an intermediary server, use the information for advertising, or sell personal information.
  >
  > **Credentials:** AI Gauge does not request or persist passwords, payment information, or authentication credentials in its own storage. It uses access tokens maintained by existing Codex and Claude Code login sessions and authentication managed by agy. Credentials are sent only to the corresponding service endpoints as required and are not logged or uploaded to the developer.
  >
  > **Third-party services:** Requests may be sent directly to OpenAI, Anthropic, or Google Antigravity services to retrieve usage information. Those services process data under their own privacy policies.
  >
  > **Local storage:** AI Gauge stores only local application preferences, including provider visibility and order, refresh interval, theme, thresholds, and window preferences. It does not maintain a remote account, analytics system, or remote database.
  >
  > **Data retention and deletion:** AI Gauge does not retain usage data on a remote server. Local preferences can be removed by uninstalling the app or clearing its local application data. Service credentials remain managed by their respective services or tools.
  >
  > **Contact:** For privacy questions, contact us at https://github.com/jmnote/aigauge/issues. Do not include passwords, access tokens, or other sensitive information in public issues.
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
  > AI Gauge is a lightweight Windows system tray widget for monitoring usage across OpenAI Codex, Anthropic Claude Code, and Google Antigravity. It shows remaining quota percentages, reset times, connection status, and the last successful update in a compact desktop window. You can show, hide, and reorder providers; configure refresh intervals and warning and critical thresholds; keep the widget above other windows; and choose a light, dark, or system theme. AI Gauge runs locally, uses existing local sign-in sessions and the locally installed agy command-line tool, and does not store a separate copy of your credentials or retain usage data on a remote server.
- **Short description**: Monitor AI service usage, quotas, reset times, and connection status from your Windows system tray.
- **Product features**:
  1. Real-time quota and reset countdown monitoring
  2. Multiple AI usage providers with visibility and ordering controls
  3. Configurable background refresh intervals
  4. Windows system tray access with light, dark, and system themes
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
- **Notes for certification**: Not provided
