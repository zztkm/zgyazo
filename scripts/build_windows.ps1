# PowerShell build script for zgyazo Windows installer
# Based on Ollama's build script implementation

param(
    [string]$Version = "1.0.0",
    [string]$InnoSetupPath = "C:\Program Files (x86)\Inno Setup 6",
    [switch]$SkipSigning = $true,
    [switch]$BuildInstaller = $true
)

$ErrorActionPreference = "Stop"

# Set up build environment
$scriptPath = Split-Path -Parent $MyInvocation.MyCommand.Path
$projectRoot = Split-Path -Parent $scriptPath
$distDir = Join-Path $projectRoot "dist"

Write-Host "Building zgyazo version $Version"

# Ensure dist directory exists
if (!(Test-Path $distDir)) {
    New-Item -ItemType Directory -Path $distDir | Out-Null
}

function Build-Zgyazo {
    Write-Host "Building zgyazo.exe..."
    
    Push-Location $projectRoot
    try {
        # Build with windowsgui flag to hide console window
        $ldflags = "-s -w -H windowsgui -X main.version=$Version"
        $env:CGO_ENABLED = "0"
        
        & go build -trimpath -ldflags $ldflags -o "$distDir\zgyazo.exe" .
        
        if ($LASTEXITCODE -ne 0) {
            throw "Failed to build zgyazo.exe"
        }
        
        Write-Host "Successfully built zgyazo.exe"
    }
    finally {
        Pop-Location
    }
}

function Sign-Executable {
    param(
        [string]$FilePath
    )
    
    if ($SkipSigning) {
        Write-Host "Skipping code signing for $FilePath"
        return
    }
    
    # Code signing implementation would go here
    # This is a placeholder for when code signing is needed
    Write-Host "Code signing not implemented yet"
}

function Build-Installer {
    if (!$BuildInstaller) {
        Write-Host "Skipping installer build"
        return
    }
    
    Write-Host "Building installer..."
    
    $issFile = Join-Path $projectRoot "installer\zgyazo.iss"
    
    if (!(Test-Path $issFile)) {
        throw "Installer script not found: $issFile"
    }
    
    # Check if Inno Setup is installed
    $iscc = Join-Path $InnoSetupPath "ISCC.exe"
    if (!(Test-Path $iscc)) {
        throw "Inno Setup not found at: $InnoSetupPath"
    }
    
    # Build the installer
    & $iscc "/DMyAppVersion=$Version" $issFile
    
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to build installer"
    }
    
    # Sign the installer if code signing is enabled
    $installerPath = Join-Path $distDir "zgyazoSetup.exe"
    if (Test-Path $installerPath) {
        Sign-Executable -FilePath $installerPath
        Write-Host "Successfully built installer: $installerPath"
    }
}

function Create-Zip {
    Write-Host "Creating standalone ZIP archive..."
    
    $zipPath = Join-Path $distDir "zgyazo-windows-amd64.zip"
    
    # Create a temporary directory for ZIP contents
    $tempDir = Join-Path $env:TEMP "zgyazo-zip-$([guid]::NewGuid())"
    New-Item -ItemType Directory -Path $tempDir | Out-Null
    
    try {
        # Copy files to temp directory
        Copy-Item "$distDir\zgyazo.exe" $tempDir
        Copy-Item "$projectRoot\LICENSE" $tempDir
        Copy-Item "$projectRoot\README.md" $tempDir
        
        # Create ZIP archive
        Compress-Archive -Path "$tempDir\*" -DestinationPath $zipPath -Force
        
        Write-Host "Successfully created ZIP archive: $zipPath"
    }
    finally {
        # Clean up temp directory
        Remove-Item -Path $tempDir -Recurse -Force
    }
}

function Show-Summary {
    Write-Host ""
    Write-Host "Build completed successfully!"
    Write-Host "Version: $Version"
    Write-Host ""
    Write-Host "Output files:"
    
    $outputs = @(
        "$distDir\zgyazo.exe",
        "$distDir\zgyazoSetup.exe",
        "$distDir\zgyazo-windows-amd64.zip"
    )
    
    foreach ($output in $outputs) {
        if (Test-Path $output) {
            $size = (Get-Item $output).Length / 1MB
            Write-Host ("  - {0} ({1:N2} MB)" -f (Split-Path -Leaf $output), $size)
        }
    }
}

# Main build process
try {
    # Build the application
    Build-Zgyazo
    
    # Sign the executable
    Sign-Executable -FilePath "$distDir\zgyazo.exe"
    
    # Build the installer
    Build-Installer
    
    # Create standalone ZIP
    Create-Zip
    
    # Show build summary
    Show-Summary
}
catch {
    Write-Error "Build failed: $_"
    exit 1
}