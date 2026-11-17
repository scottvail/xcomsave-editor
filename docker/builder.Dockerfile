# SPDX-License-Identifier: GPL-2.0-or-later
# Copyright (C) 2026 Scott Vail
#
# Build environment for the GUI editor (Fyne needs cgo). Used by the Makefile
# for Linux builds (one image per architecture) and for the Windows build
# (MinGW-w64 cross compiler). It also carries a cross compiler and libraries
# for the other Linux architecture (amd64 <-> arm64), so make.ps1 can build
# every Linux editor from one container.
FROM golang:1.24-bookworm

RUN set -e; \
    native=$(dpkg --print-architecture); \
    case "$native" in \
        amd64) other=arm64; cross=gcc-aarch64-linux-gnu ;; \
        arm64) other=amd64; cross=gcc-x86-64-linux-gnu ;; \
        *) echo "unsupported architecture $native" >&2; exit 1 ;; \
    esac; \
    dpkg --add-architecture "$other"; \
    apt-get update; \
    apt-get install -y --no-install-recommends \
        gcc libc6-dev pkg-config gcc-mingw-w64-x86-64 "$cross"; \
    for a in "$native" "$other"; do \
        apt-get install -y --no-install-recommends \
            libc6-dev:$a libgl1-mesa-dev:$a libx11-dev:$a libxcursor-dev:$a \
            libxrandr-dev:$a libxinerama-dev:$a libxi-dev:$a libxxf86vm-dev:$a; \
    done; \
    rm -rf /var/lib/apt/lists/*

# Tells the Makefile it is running inside this image (see make.ps1).
ENV XCOM_BUILDER=1
