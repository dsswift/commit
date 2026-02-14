# Commit Tool Installer for Windows
# Usage: iwr -useb https://raw.githubusercontent.com/dsswift/commit/main/scripts/install.ps1 | iex

$ErrorActionPreference = "Stop"

# Configuration
$Repo = "dsswift/commit"
$InstallDir = "$env:USERPROFILE\.local\bin"
$ConfigDir = "$env:USERPROFILE\.commit-tool"

function Write-ColorOutput($ForegroundColor) {
    $fc = $host.UI.RawUI.ForegroundColor
    $host.UI.RawUI.ForegroundColor = $ForegroundColor
    if ($args) {
        Write-Output $args
    }
    $host.UI.RawUI.ForegroundColor = $fc
}

function Get-LatestVersion {
    $release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest"
    return $release.tag_name
}

function Verify-Checksum {
    param(
        [string]$Version,
        [string]$Filename,
        [string]$TargetPath
    )

    $checksumUrl = "https://github.com/$Repo/releases/download/$Version/checksums.txt"

    try {
        $checksumContent = (Invoke-WebRequest -Uri $checksumUrl -UseBasicParsing).Content
    }
    catch {
        Write-ColorOutput Yellow "Warning: checksums.txt not available for $Version, skipping verification"
        return
    }

    # Parse checksums.txt: "<hash>  <filename>" format
    $expectedHash = $null
    foreach ($line in $checksumContent -split "`n") {
        $line = $line.Trim()
        if ($line -eq "") { continue }
        $parts = $line -split '\s+'
        if ($parts.Length -eq 2 -and $parts[1] -eq $Filename) {
            $expectedHash = $parts[0]
            break
        }
    }

    if (-not $expectedHash) {
        Write-ColorOutput Yellow "Warning: no checksum entry for $Filename, skipping verification"
        return
    }

    $actualHash = (Get-FileHash -Path $TargetPath -Algorithm SHA256).Hash

    if ($actualHash -ine $expectedHash) {
        Remove-Item -Path $TargetPath -Force -ErrorAction SilentlyContinue
        Write-ColorOutput Red "Checksum mismatch: expected $expectedHash, got $actualHash"
        exit 1
    }

    Write-ColorOutput Green "Checksum verified"
}

function Install-CommitTool {
    Write-Output "🚀 Installing Commit Tool..."
    Write-Output ""

    # Detect architecture
    $arch = if ([Environment]::Is64BitOperatingSystem) { "amd64" } else { "386" }
    Write-Output "Detected architecture: windows-$arch"

    # Get latest version
    $version = Get-LatestVersion
    Write-Output "Latest version: $version"

    # Download URL
    $filename = "commit-windows-$arch.exe"
    $url = "https://github.com/$Repo/releases/download/$version/$filename"
    Write-Output "Downloading from: $url"

    # Create install directory
    if (!(Test-Path $InstallDir)) {
        New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    }

    # Download binary
    $targetPath = Join-Path $InstallDir "commit.exe"
    try {
        Invoke-WebRequest -Uri $url -OutFile $targetPath -UseBasicParsing
    }
    catch {
        Write-ColorOutput Red "Download failed: $_"
        exit 1
    }

    # Verify checksum
    Verify-Checksum -Version $version -Filename $filename -TargetPath $targetPath

    Write-ColorOutput Green "Installed to: $targetPath"

    # Create config directory
    if (!(Test-Path $ConfigDir)) {
        New-Item -ItemType Directory -Path $ConfigDir -Force | Out-Null
    }

    # Create config template
    $envFile = Join-Path $ConfigDir ".env"
    if (!(Test-Path $envFile)) {
        $configContent = @"
# Commit Tool Configuration
# Documentation: https://github.com/dsswift/commit#configuration

# ═══════════════════════════════════════════════════════════════════════════════
# PROVIDER SELECTION (required)
# ═══════════════════════════════════════════════════════════════════════════════
# Choose one: anthropic | openai | grok | gemini | azure-foundry
COMMIT_PROVIDER=

# ═══════════════════════════════════════════════════════════════════════════════
# PUBLIC CLOUD API KEYS (use one matching your provider)
# ═══════════════════════════════════════════════════════════════════════════════
ANTHROPIC_API_KEY=
OPENAI_API_KEY=
GROK_API_KEY=
GEMINI_API_KEY=

# ═══════════════════════════════════════════════════════════════════════════════
# AZURE AI FOUNDRY (private cloud - optional)
# ═══════════════════════════════════════════════════════════════════════════════
# For organizations using Azure-hosted models
AZURE_FOUNDRY_ENDPOINT=
AZURE_FOUNDRY_API_KEY=
AZURE_FOUNDRY_DEPLOYMENT=

# ═══════════════════════════════════════════════════════════════════════════════
# OPTIONAL SETTINGS
# ═══════════════════════════════════════════════════════════════════════════════
# Override default model for your provider
# COMMIT_MODEL=claude-3-5-sonnet

# Always preview without committing (useful for testing)
# COMMIT_DRY_RUN=true
"@
        Set-Content -Path $envFile -Value $configContent
        Write-ColorOutput Green "Created config template: $envFile"
    }
    else {
        Write-ColorOutput Yellow "Config file already exists: $envFile"
    }

    # Check PATH
    $userPath = [Environment]::GetEnvironmentVariable("PATH", "User")
    if ($userPath -notlike "*$InstallDir*") {
        Write-ColorOutput Yellow "Adding $InstallDir to PATH..."
        $newPath = "$userPath;$InstallDir"
        [Environment]::SetEnvironmentVariable("PATH", $newPath, "User")
        $env:PATH = "$env:PATH;$InstallDir"
        Write-ColorOutput Green "Added to PATH"
    }
    else {
        Write-ColorOutput Green "$InstallDir is already in PATH"
    }

    Write-Output ""
    Write-ColorOutput Green "✅ Installation complete!"
    Write-Output ""
    Write-Output "Next steps:"
    Write-Output "  1. Edit $envFile to configure your LLM provider"
    Write-Output "  2. Run 'commit --version' to verify installation"
    Write-Output "  3. Run 'commit' in a git repository to create smart commits"
    Write-Output ""
    Write-Output "Documentation: https://github.com/$Repo#readme"
}

Install-CommitTool
