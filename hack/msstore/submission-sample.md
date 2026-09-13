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
- **Description**: See [`submission-sample.yaml`](submission-sample.yaml) (`Listings.en-us.BaseListing.Description`).
- **What's new in this version**: See [`submission-sample.yaml`](submission-sample.yaml) (`Listings.en-us.BaseListing.ReleaseNotes`).
- **Product features**: See [`submission-sample.yaml`](submission-sample.yaml) (`Listings.en-us.BaseListing.Features`).
- **Screenshots**: 2 Desktop images
  1. AI Gauge usage dashboard in dark mode.
  2. AI Gauge usage dashboard in light mode.
- **Supplemental fields**:
  - **Short description**: See [`submission-sample.yaml`](submission-sample.yaml) (`Listings.en-us.BaseListing.ShortDescription`).
  - **Keywords**: See [`submission-sample.yaml`](submission-sample.yaml) (`Listings.en-us.BaseListing.Keywords`).
  - **Copyright and trademark info**: © 2026 jmnote. Licensed under the Apache License 2.0
  - **Developed by**: jmnote

## Submission options

- **Publishing**: Publish this submission as soon as it passes certification
- **Restricted capability justification — runFullTrust**:
  > AI Gauge is a Wails-based Windows desktop application. It requires runFullTrust because it runs as a native Win32 process, provides a system tray UI, and invokes the locally installed agy CLI to retrieve usage data. The capability is required for the application's core desktop functionality.

## Supplemental info

### Additional Testing Info

#### Notes for Certification

See [`submission-sample.yaml`](submission-sample.yaml) (`NotesForCertification`).

#### Credentials

- None
