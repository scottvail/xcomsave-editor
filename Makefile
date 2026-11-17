# SPDX-License-Identifier: GPL-2.0-or-later
# Copyright (C) 2026 Scott Vail
#
# Builds the xcom2json / json2xcom command-line tools and the GUI editor for
# every supported platform into build/dist/<os>/<arch>/bin.
#
#   make                     build everything (same as "make dist")
#   make tools               command-line tools only (pure Go, no Docker)
#   make editor              GUI editor only
#   make dist-linux_arm64    tools + editor for one platform (<os>_<arch>)
#   make tools-linux_arm64   tools for one platform
#   make editor-linux_arm64  editor for one platform
#   make test                run the tests
#   make test-ui             run the GUI tests (needs the editor's cgo/GL
#                            libraries, e.g. inside the builder container)
#   make samples            rebuild example/{eu,ew}-sample from the sources
#   make clean               remove build/dist
#
# The editor uses Fyne, which needs cgo and therefore a C toolchain for each
# target:
#   darwin_*   built on the host with clang (requires a macOS host with the
#              Xcode command line tools)
#   linux_*    built in a Debian container for that architecture
#              (requires Docker; non-native architectures run emulated)
#   windows_*  built in a container with the MinGW-w64 cross compiler
#
# On Windows, with only Docker Desktop installed (no make, Go or gcc), run
# make inside the builder container with the wrapper script instead:
#   .\make.ps1                       Windows + Linux (amd64, arm64) tools and
#                                    editor, macOS tools only
#   .\make.ps1 test                  any other target
# The macOS editor can't be built that way: it needs Apple's SDK.

GO        ?= go
DOCKER    ?= docker
DIST      := build/dist
TOOLS     := xcom2json json2xcom
TOOLSMOD  := github.com/scottvail/go-xcomsave
EDITOR    := xcomsave-editor
EDITORPKG := ./cmd/$(EDITOR)
LDFLAGS   := -s -w
IMAGE     := xcom-editor-builder
DOCKERDIR := docker
CACHE     := $(CURDIR)/build/.cache

# <os>_<arch> pairs. Windows is amd64 only.
PLATFORMS := windows_amd64 linux_amd64 linux_arm64 darwin_amd64 darwin_arm64

HOST_OS := $(shell $(GO) env GOHOSTOS)

# Set (by the builder image) when make itself runs inside the builder
# container, e.g. via make.ps1 on Windows. Docker isn't available there, so
# the Windows editor is built directly with the MinGW-w64 cross compiler.
XCOM_BUILDER ?=

# Sets $$os and $$arch from the pattern stem "<os>_<arch>" in a recipe.
SPLIT = os=$(word 1,$(subst _, ,$*)); arch=$(word 2,$(subst _, ,$*))

# docker run prefix for the builder image for linux/$(1), with the source
# tree mounted and Go caches kept in build/.cache. $(2) names the cache.
DOCKER_RUN = $(DOCKER) run --rm --platform linux/$(1) \
	-u $$(id -u):$$(id -g) -e HOME=/tmp \
	-e GOCACHE=/cache/go-build-$(2) -e GOMODCACHE=/cache/mod -e GOFLAGS=-buildvcs=false -e GOWORK=off \
	-v "$(CURDIR)":/src -v "$(CACHE)":/cache -w /src

# The per-platform targets are deliberately not .PHONY: make skips pattern
# rules for phony targets. They never create a file of that name, so they
# always run.
.PHONY: all dist tools editor test test-ui samples clean

all: dist

dist: tools editor

tools: $(addprefix tools-,$(PLATFORMS))

editor: $(addprefix editor-,$(PLATFORMS))

dist-%: tools-% editor-%
	@:

# The tools are pure Go, so CGO_ENABLED=0 cross-compiles them from any host.
tools-%:
	@$(SPLIT); ext=""; [ "$$os" = windows ] && ext=.exe; \
	out=$(DIST)/$$os/$$arch/bin; mkdir -p $$out; \
	for t in $(TOOLS); do \
		echo "  $$out/$$t$$ext"; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch $(GO) build -trimpath \
			-ldflags "$(LDFLAGS)" -o $$out/$$t$$ext $(TOOLSMOD)/cmd/$$t || exit 1; \
	done

editor-darwin_%:
	@if [ "$(HOST_OS)" != darwin ]; then \
		echo "error: the macOS editor can only be built on a macOS host" >&2; exit 1; fi
	@out=$(DIST)/darwin/$*/bin; mkdir -p $$out; echo "  $$out/$(EDITOR)"; \
	cc_arch=$*; [ "$*" = amd64 ] && cc_arch=x86_64; \
	CGO_ENABLED=1 GOOS=darwin GOARCH=$* CC="clang -arch $$cc_arch" \
		$(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $$out/$(EDITOR) $(EDITORPKG) 2>&1 \
		| grep -v -e "ignoring duplicate libraries" -e "^# " || true; \
	test -x $$out/$(EDITOR)

# Inside the builder container the compilers are used directly: the image has
# gcc for both Linux architectures (one native, one cross) and MinGW-w64.
# Outside it, Linux editors build in the image for their own architecture and
# the MinGW-w64 cross compiler runs in the image for the Docker host's.
ifneq ($(XCOM_BUILDER),)
LINUX_TRIPLE_amd64 := x86_64-linux-gnu
LINUX_TRIPLE_arm64 := aarch64-linux-gnu

editor-linux_%:
	@triple=$(LINUX_TRIPLE_$*); \
	if [ -z "$$triple" ]; then echo "error: unsupported Linux architecture $*" >&2; exit 1; fi; \
	out=$(DIST)/linux/$*/bin; mkdir -p $$out; echo "  $$out/$(EDITOR)"; \
	CGO_ENABLED=1 GOOS=linux GOARCH=$* CC=$$triple-gcc \
	PKG_CONFIG_LIBDIR=/usr/lib/$$triple/pkgconfig:/usr/share/pkgconfig \
		$(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $$out/$(EDITOR) $(EDITORPKG)

editor-windows_amd64:
	@out=$(DIST)/windows/amd64/bin; mkdir -p $$out; echo "  $$out/$(EDITOR).exe"; \
	CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=x86_64-w64-mingw32-gcc \
		$(GO) build -trimpath -ldflags "$(LDFLAGS) -H=windowsgui" -o $$out/$(EDITOR).exe $(EDITORPKG)
else
editor-linux_%: builder-image-%
	@out=$(DIST)/linux/$*/bin; mkdir -p $$out "$(CACHE)"; echo "  $$out/$(EDITOR)"; \
	$(call DOCKER_RUN,$*,linux-$*) $(IMAGE):$* \
		go build -trimpath -ldflags "$(LDFLAGS)" -o $$out/$(EDITOR) $(EDITORPKG)

editor-windows_amd64:
	@arch=$$($(DOCKER) version -f '{{.Server.Arch}}'); \
	$(MAKE) --no-print-directory builder-image-$$arch || exit 1; \
	out=$(DIST)/windows/amd64/bin; mkdir -p $$out "$(CACHE)"; echo "  $$out/$(EDITOR).exe"; \
	$(call DOCKER_RUN,$$arch,windows) \
		-e CGO_ENABLED=1 -e GOOS=windows -e GOARCH=amd64 -e CC=x86_64-w64-mingw32-gcc \
		$(IMAGE):$$arch \
		go build -trimpath -ldflags "$(LDFLAGS) -H=windowsgui" -o $$out/$(EDITOR).exe $(EDITORPKG)
endif

builder-image-%:
	@echo "  docker image $(IMAGE):$*"; \
	$(DOCKER) build -q --platform linux/$* -t $(IMAGE):$* \
		-f $(DOCKERDIR)/builder.Dockerfile $(DOCKERDIR) >/dev/null

test:
	$(GO) test ./internal/...

# The GUI tests drive the editor headlessly on example/{eu,ew}-source.
test-ui:
	$(GO) test -count=1 -run TestUI $(EDITORPKG)

# The example saves the tests use: built from example/{eu,ew}-source (as the
# game saved them) by internal/save/gen_samples_test.go.
samples:
	$(GO) test -count=1 -tags samples -run TestGenerateSamples ./internal/save/

clean:
	rm -rf $(DIST)

build-clean:
	rm -rf build