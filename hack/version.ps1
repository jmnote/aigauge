# Shared version-normalization helper, dot-sourced by build.ps1,
# hack/package-msix.ps1, and hack/prepare-windows-resources.ps1 so the
# semantic-version parsing rules live in exactly one place.
function Resolve-Version {
    param([string]$Requested)
    if ([string]::IsNullOrWhiteSpace($Requested)) {
        $Requested = "0.0.0"
    }
    $value = $Requested.TrimStart('v', 'V')
    if ($value -notmatch '^\d+\.\d+\.\d+(\.\d+)?(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$') {
        throw "Version must be semantic version text such as 0.1.1, 0.1.1-beta1, or 0.1.1-beta1+build5. Received: $Requested"
    }
    $value = ($value -split '[-+]', 2)[0]
    $parts = @($value.Split('.'))
    while ($parts.Count -lt 4) { $parts += '0' }
    return ($parts -join '.')
}
