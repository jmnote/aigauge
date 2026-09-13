# Microsoft Store Publishing & Partner Center Administration

This document covers Microsoft Store publication, Partner Center configuration, repository secrets, and certification smoke tests for AI Gauge.

---

## 1. Publishing Workflow Overview

When a stable release tag (e.g. `v0.6.2`) is pushed to GitHub, the Release workflow (`.github/workflows/release.yml`) builds the application, creates a GitHub Release, and publishes the package to Microsoft Store via the Microsoft Store Developer CLI (`msstore`).

### Release Tag Format
- Stable tags follow semantic versioning: `vX.Y.Z` (e.g. `v0.6.2`).
- The four-part MSIX package version is derived from the release tag (e.g. `0.6.2.0`).

```powershell
git tag v0.6.2
git push origin v0.6.2
```

---

## 2. Partner Center Credentials & Secrets

The release workflow uses Microsoft's [`microsoft-store-apppublisher`](https://github.com/microsoft/microsoft-store-apppublisher) action to authenticate against Partner Center. The following repository or environment secrets are required:

| Secret | Description |
|---|---|
| `AZURE_AD_TENANT_ID` | Azure Active Directory Tenant ID associated with the Partner Center account. |
| `SELLER_ID` | Partner Center Seller ID. |
| `AZURE_AD_APPLICATION_CLIENT_ID` | Azure AD Application (client) ID with Store submission permissions. |
| `AZURE_AD_APPLICATION_SECRET` | Azure AD Application client secret key. |

These credentials are configured locally via `msstore reconfigure` or loaded from `hack/submission.env` for CLI tasks.

---

## 3. Metadata Overrides & Validation

Store submission overrides and certification test instructions are maintained in:
- [`submission-overrides.yaml`](submission-overrides.yaml): Overrides applied to the active draft submission during the release workflow.
- [`submission-sample.yaml`](submission-sample.yaml): Reference export snapshot demonstrating the full Partner Center submission schema.
- [`submission-sample.md`](submission-sample.md): Human-readable specification and listing overview.

### How Overrides Work
1. The release workflow runs `msstore publish` with the draft submission option.
2. If `hack/msstore/submission-overrides.yaml` exists, only the declared fields (such as release notes, feature bullets, or certification notes) are updated in the draft.
3. The workflow commits the submission. This keeps Partner Center metadata version-controlled alongside application code while preserving undeclared Store dashboard settings.

### Local CLI Verification
Use `hack/msstore/submission.mjs` or `build.ps1` tasks to inspect and validate submission data:

```powershell
# Validate submission overrides syntax and schema constraints
.\build.ps1 submission-validate

# Retrieve the current submission JSON from Partner Center (requires submission.env)
.\build.ps1 submission-get

# Convert the fetched submission JSON into submission-sample.yaml
.\build.ps1 submission-yaml
```

---

## 4. Package Identity & Capabilities

- **Identity & Publisher**:
  The package uses the reserved Partner Center identity in `Package.appxmanifest`. Do not replace the `Identity Name` or `Publisher` with an arbitrary certificate or publisher string. Microsoft Store handles package signing automatically during ingestion.
- **Restricted Capability (`runFullTrust`)**:
  AI Gauge requires `runFullTrust` because it runs as a native Win32/Wails desktop application, provides a system tray UI, and invokes local CLI tools (`agy`) to retrieve quota data.

---

## 5. Pre-Submission Certification Smoke Test

Before submitting a major update to certification, run these quick sanity checks on a clean test device:

1. **First-run setup experience**:
   - Run `.\build.ps1 ai-backup` to simulate a clean device with no CLI tools or credentials installed.
   - Launch AI Gauge: Verify that missing providers display clear, polite setup guidance without errors or crash dialogs.
2. **Sample preview mode**:
   - Click **Preview with sample data**: Confirm that sample dashboard gauges, percentages, and reset times render cleanly without accounts or network requests.
3. **Settings & customization**:
   - Open Settings (gear icon): Test light/dark/system themes, threshold sliders, window width resizing, and global hotkeys.
4. **System tray behavior**:
   - Test minimize-to-tray, tray icon click to restore, and right-click context menu options.
5. **Restore environment**:
   - Run `.\build.ps1 ai-restore` to restore local development credentials.

