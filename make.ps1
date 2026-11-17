# SPDX-License-Identifier: GPL-2.0-or-later
# Copyright (C) 2026 Scott Vail
#
# Runs make inside the builder container, so building on Windows needs only
# Docker Desktop (Linux containers) - no make, Go or C compiler on the host.
#
#   .\make.ps1                 everything that can be built from Windows:
#                                Windows amd64, Linux amd64 + arm64: tools + editor
#                                macOS amd64 + arm64:                tools only
#   .\make.ps1 dist-linux_arm64  one platform (any Makefile target works)
#   .\make.ps1 test            run the tests
#
# Output goes to build\dist\<os>\<arch>\bin. The macOS editor isn't available
# this way: cross-compiling it needs Apple's macOS SDK, so build it on a Mac.

$ErrorActionPreference = 'Stop'

$image = 'xcom-editor-builder'
$root  = $PSScriptRoot
$cache = Join-Path $root 'build\.cache'

$default = @(
    'dist-windows_amd64'
    'dist-linux_amd64'
    'dist-linux_arm64'
    'tools-darwin_amd64'
    'tools-darwin_arm64'
)
# "all" and "dist" would include the macOS editor, so map them to the default.
$targets = @(if ($args.Count -eq 0 -or ($args.Count -eq 1 -and $args[0] -in 'all', 'dist')) {
    $default
} else {
    $args
})

# Windows won't let the build overwrite a program that is running, so stop
# early (with a clear message) if one from build\dist is open.
$dist = Join-Path $root 'build\dist'
$running = @()
if (@($targets | Where-Object { $_ -notin 'test', 'test-ui', 'samples' }).Count -gt 0) {
    $running = @(Get-Process -ErrorAction SilentlyContinue |
        Where-Object { $_.Path -and $_.Path.StartsWith("$dist\", [StringComparison]::OrdinalIgnoreCase) })
}
if ($running.Count -gt 0) {
    $list = ($running | ForEach-Object { "  $($_.Path) (process $($_.Id))" }) -join "`n"
    Write-Host -ForegroundColor Red "error: close these programs first - Windows won't let the build overwrite them while they run:`n$list"
    exit 1
}

$arch = (docker version -f '{{.Server.Arch}}')
if ($LASTEXITCODE -ne 0) { throw 'docker is not available - is Docker Desktop running?' }

Write-Host "  docker image ${image}:$arch"
docker build -q --platform "linux/$arch" -t "${image}:$arch" `
    -f (Join-Path $root 'docker\builder.Dockerfile') (Join-Path $root 'docker') | Out-Null
if ($LASTEXITCODE -ne 0) { throw 'failed to build the builder image' }

New-Item -ItemType Directory -Force $cache | Out-Null

docker run --rm --platform "linux/$arch" `
    -e HOME=/tmp `
    -e GOCACHE=/cache/go-build-windows -e GOMODCACHE=/cache/mod -e GOFLAGS=-buildvcs=false -e GOWORK=off `
    -v "${root}:/src" -v "${cache}:/cache" -w /src `
    "${image}:$arch" make --no-print-directory @targets
exit $LASTEXITCODE
