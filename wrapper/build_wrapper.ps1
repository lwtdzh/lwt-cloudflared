<#
.SYNOPSIS
    Binary Wrapper Creator for Windows - Privacy Protection for Executables

.DESCRIPTION
    Windows analogue of create_wrapper.py. Produces a self-contained .exe that:
    - Encrypts the input binary with XOR (random 32-byte key per build)
    - Embeds the encrypted payload as a .NET manifest resource inside a small
      .NET Framework 4 loader (compiled with the in-box csc.exe, no external
      toolchain required)
    - At runtime: reads the resource, XOR-decrypts in memory, drops a hidden
      GUID-named file under %TEMP%, exec's it with all forwarded args, waits
      for it to exit, then deletes the temp file

    Mapping to the Linux wrapper (create_wrapper.py):
      Linux memfd_create        -> not available on Windows; skipped
      Linux /dev/shm + rm       -> %TEMP%\_<guid>.exe (hidden) + delete in finally
      bash + python + perl deps -> none (single self-contained .exe)
      base64 of XORed payload   -> binary .NET resource (no base64 needed)

    Caveats:
      - This is NOT truly fileless. The decrypted exe exists on disk for the
        lifetime of the child process. A hard-kill of the wrapper leaves the
        temp file behind (same property as the /dev/shm fallback path).
      - Self-decrypting loaders are a textbook packer pattern. Defender,
        SmartScreen, and third-party AVs may quarantine the output even
        though the inner payload is benign. Sign the wrapper or whitelist
        its path if that's a problem.

.PARAMETER InputFile
    Path to the .exe to wrap.

.PARAMETER OutputFile
    Path for the wrapped .exe. Defaults to "wrapped_<basename>" next to the input.

.EXAMPLE
    .\build_wrapper.ps1 .\cfd-win64.exe .\cfd-win64-wrapped.exe

.EXAMPLE
    .\build_wrapper.ps1 C:\path\to\binary.exe

.NOTES
    Requirements:
      - Windows with .NET Framework 4 (csc.exe in
        C:\Windows\Microsoft.NET\Framework64\v4.0.30319)
      - PowerShell 5+
      - loader.cs in the same directory as this script
#>
[CmdletBinding()]
param(
    [Parameter(Mandatory = $true, Position = 0)]
    [string]$InputFile,

    [Parameter(Position = 1)]
    [string]$OutputFile
)

$ErrorActionPreference = 'Stop'

if (-not (Test-Path -LiteralPath $InputFile)) {
    Write-Error "[-] Input file not found: $InputFile"
    exit 1
}
$InputFile = (Resolve-Path -LiteralPath $InputFile).Path

if (-not $OutputFile) {
    $base = [IO.Path]::GetFileName($InputFile)
    $OutputFile = Join-Path ([IO.Path]::GetDirectoryName($InputFile)) "wrapped_$base"
}

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$loaderTpl = Join-Path $scriptDir 'loader.cs'
if (-not (Test-Path -LiteralPath $loaderTpl)) {
    Write-Error "[-] loader.cs not found next to build_wrapper.ps1: $loaderTpl"
    exit 1
}

$csc = "$env:WINDIR\Microsoft.NET\Framework64\v4.0.30319\csc.exe"
if (-not (Test-Path -LiteralPath $csc)) {
    Write-Error "[-] csc.exe not found: $csc (install .NET Framework 4)"
    exit 1
}

$work = Join-Path $env:TEMP ("wrap_build_" + [Guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $work -Force | Out-Null
$payload  = Join-Path $work 'p'
$loaderCs = Join-Path $work 'loader_gen.cs'

try {
    Write-Host "[*] Creating Windows fileless-style wrapper..."
    Write-Host "[*] Input:  $InputFile"
    Write-Host "[*] Output: $OutputFile"

    Write-Host "[*] Reading binary..."
    $data = [IO.File]::ReadAllBytes($InputFile)
    $origSize = $data.Length
    Write-Host ("[+] Original size: {0} bytes ({1:N1} MB)" -f $origSize, ($origSize / 1MB))

    Write-Host "[*] Generating 32-byte key..."
    $rng = [Security.Cryptography.RandomNumberGenerator]::Create()
    try {
        $key = New-Object byte[] 32
        $rng.GetBytes($key)
    } finally { $rng.Dispose() }

    Write-Host "[*] Encrypting (XOR)..."
    $kl = $key.Length
    for ($i = 0; $i -lt $data.Length; $i++) {
        $data[$i] = $data[$i] -bxor $key[$i % $kl]
    }

    Write-Host "[*] Writing encrypted resource..."
    [IO.File]::WriteAllBytes($payload, $data)
    $data = $null
    [GC]::Collect()

    Write-Host "[*] Generating loader source..."
    $keyLits = ($key | ForEach-Object { '0x{0:x2}' -f $_ }) -join ', '
    $tpl = [IO.File]::ReadAllText($loaderTpl)
    if ($tpl.IndexOf('__KEY_BYTES__') -lt 0) {
        throw "loader.cs is missing the __KEY_BYTES__ placeholder"
    }
    $tpl = $tpl.Replace('__KEY_BYTES__', $keyLits)
    [IO.File]::WriteAllText($loaderCs, $tpl, (New-Object Text.UTF8Encoding($false)))

    Write-Host "[*] Compiling with csc.exe..."
    if (Test-Path -LiteralPath $OutputFile) { Remove-Item -LiteralPath $OutputFile -Force }
    $cscArgs = @(
        '/nologo',
        '/target:exe',
        '/platform:x64',
        '/optimize+',
        '/debug-',
        "/out:$OutputFile",
        "/resource:$payload,p",
        $loaderCs
    )
    & $csc @cscArgs
    if ($LASTEXITCODE -ne 0) { throw "csc.exe failed with exit code $LASTEXITCODE" }

    $f = Get-Item -LiteralPath $OutputFile
    $sha = (Get-FileHash -LiteralPath $OutputFile -Algorithm SHA256).Hash

    Write-Host ""
    Write-Host ("=" * 60)
    Write-Host "[+] Wrapper created successfully!"
    Write-Host ("=" * 60)
    Write-Host "[+] Output: $OutputFile"
    Write-Host ("[+] Size: {0} bytes ({1:N1} MB)" -f $f.Length, ($f.Length / 1MB))
    Write-Host "[+] SHA256: $sha"
    Write-Host ""
    Write-Host "Execution behavior:"
    Write-Host "  - Decrypts payload in memory at startup"
    Write-Host "  - Drops hidden %TEMP%\_<guid>.exe, exec's it with forwarded args"
    Write-Host "  - Deletes temp file after the inner process exits"
    Write-Host ""
    Write-Host "Usage:"
    Write-Host "  $([IO.Path]::GetFileName($OutputFile)) [args...]"
}
finally {
    if (Test-Path -LiteralPath $work) {
        Remove-Item -LiteralPath $work -Recurse -Force -ErrorAction SilentlyContinue
    }
}
