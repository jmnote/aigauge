# Microsoft Store submission notes

These instructions apply to files under `hack/msstore/`.

## Store submission files

- `submission-overrides.yaml` contains the listing copy, feature bullets, release
  notes, and certification instructions applied to the Store submission. Keep the
  overrides accurate and concise: describe implemented, reviewable behavior and
  include only the certification steps needed to verify it.
- When a provider is added or its authentication/data handling changes, also check
  [`docs/privacy-policy.md`](../../docs/privacy-policy.md) covers it - Store
  certification checks that the privacy policy matches what the app actually does.
- `submission-snapshot.yaml` and `submission-snapshot.md` record the previous Store
  submission and are updated manually by the maintainer. Changes to them may be
  included in a PR, but agents must not edit either file.
