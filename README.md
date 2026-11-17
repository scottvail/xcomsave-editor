# XCOM: Enemy Within Save Editor

A cross-platform (Windows, macOS, Linux) desktop app for editing XCOM: Enemy
Unknown and Enemy Within saved games. It opens the game's own save files
(e.g. `save62`) directly, and also JSON files produced by `xcom2json`, and
writes them back in the same format, ready to load in the game.

What it can change:

- **Campaign:** cash, difficulty, lowest difficulty and Ironman.
- **Barracks:** each soldier's name, nickname, class (including MEC Trooper
  in Enemy Within), rank, experience, stats, psi rank and gift, load out,
  wounds and (Enemy Within) gene mods. One-click actions heal every soldier
  (including those in surgery), give everyone the psi gift, build a
  "God Squad" of maxed-out Colonels, hire new soldiers and rush hires on
  their way.
- **Hangar:** each interceptor's callsign, type, weapon, stats, hit points,
  down time and home continent; order new interceptors for any continent,
  fulfill pending orders at once and repair every damaged craft.
- **Storage:** the number of each weapon, armor, item, S.H.I.V., aircraft,
  resource, alien artifact and corpse held (Enemy Unknown saves for now).
- **Research:** the number of scientists, and finishing the current
  research project.
- **Engineering:** the number of engineers, finishing construction and
  facility projects, completing item and Foundry projects, and deploying
  launched satellites at once.

The Barracks and Storage editors are described in more detail below.

Opening a save shows a landing page with the save's details (description,
date, game version, autosave flag), editable campaign settings (cash,
difficulty, lowest difficulty, Ironman) and a button for each editor. **Barracks** lists every
`Command1.TheWorld:PersistentLevel.XGStrategySoldier_*` entry in
`checkpoints[].checkpoint_table` (first name, last name, rank, class, wounded,
Psi gift and, for Enemy Within, the number of gene mods);
click a soldier to open their editor, where you can change:

| Field       | Property                    |
|-------------|-----------------------------|
| First Name  | `m_kSoldier.strFirstName`   |
| Last Name   | `m_kSoldier.strLastName`    |
| Nickname    | `m_kSoldier.strNickName`    |
| Rank        | `m_kSoldier.iRank` (0 Rookie – 7 Colonel) |
| Class       | `m_kSoldier.kClass`: `strName`, `eType` and `eWeaponType`, changed together (None, Sniper, Assault, Support, Heavy) |
| Health      | `m_kChar.aStats[0]`         |
| Aim         | `m_kChar.aStats[1]`         |
| Defense     | `m_kChar.aStats[2]`         |
| Travel      | `m_kChar.aStats[3]` (mobility) |
| Will        | `m_kChar.aStats[7]`         |
| Psi Rank    | `m_kSoldier.iPsiRank`       |
| Experience  | `m_kSoldier.iXP`            |
| Psi Experience | `m_kSoldier.iPsiXP`      |
| Psi: Gifted | `m_kChar.bHasPsiGift`       |
| Psi: Tested | `m_bPsiTested`              |
| Psi: Forced | `bForcePsiGift`             |
| Armor       | `m_kChar.kInventory.iArmor` (a MEC Trooper wears Base Augments or a MEC suit: on save the suit taken off is put back in storage, and the one put on is claimed from storage, or added to it if none is free) |
| Pistol      | `m_kChar.kInventory.iPistol` (not for Heavies; cleared to 0 for a Heavy when its load out or class is edited) |
| Weapon      | `m_kChar.kInventory.arrLargeItems[0]` (limited to the class's weapons; changing class swaps in the new class's weapon of the same tier) |
| Rocket      | `m_kChar.kInventory.arrLargeItems[1]` (Heavy only; `iNumLargeItems` is set to 2 for Heavies, 1 otherwise) |
| Item 1      | `m_kChar.kInventory.arrSmallItems[0]` |
| Item 2      | `m_kChar.kInventory.arrSmallItems[1]` (`iNumSmallItems` is set to 2 when non-zero, otherwise 1) |
| Turns Out   | `m_iTurnsOut` (wounded; shown in the soldier list's Wounded column) |
| Gene Mods   | `m_kChar.aUpgrades` flags (Enemy Within only; not for MECs): per body part None or one of its two mods — Brain: Neural Damping (28) / Neural Feedback (29); Eyes: Hyper-Reactive Pupils (30) / Depth Perception (38); Chest: Secondary Heart (4) / Adrenal Neurosympathy (20); Skin: Bioelectric Skin (42) / Mimetic Skin (95); Legs: Muscle Fiber Density (50) / Adaptive Bone Marrow (45) |


Load out items (Armor, Pistol, Weapon, Rocket, Item 1, Item 2) are chosen by
name from the lists in [internal/save/items.go](internal/save/items.go);
Pistol, Rocket and items can also be *None* (0). Pistols, weapons and rockets
are numbered the same in every save, but armor and items are not, so the
lists are picked from the save's version (shown on the landing page):
Enemy Unknown (0x0f) or Enemy Within (0x10, Android 0x13), which also has
the EXALT weapons and rocket and items such as the Mimic Beacon. A value not
in the list (for example from a mod) is shown as *Unlisted (N)* and kept
unless you pick another.

**Storage** opens a page with the number of each item in XCOM's
storage (the `XGStorage` entry's `m_arrItems`), grouped into weapons, armor,
equipment, S.H.I.V.s, aircraft, resources, alien artifacts and corpses. Items are
numbered from 1, so element *N* is `m_arrItems[N-1]`; the
mapping is in [internal/save/items.go](internal/save/items.go). The mapping uses
Enemy Unknown's item numbers, so storage editing is only available for
Enemy Unknown (version 0x0f) saves for now.

## Workflow

1. Open a save (Ctrl/Cmd+O). PC saves are in
   `Documents/My Games/XCOM - Enemy Within/XComGame/SaveData` (or
   `XCOM - Enemy Unknown`); they have no file extension. Geoscape saves only;
   saves made during a tactical mission are not supported.
2. Click **Barracks**, pick a soldier and make your changes. Going back
   to the list (or the landing page) keeps them; edit as many soldiers as
   you like, then **Save** (Ctrl/Cmd+S) from any page. The file is written in the format it was
   opened in, so a game save can be loaded by the game straight away.
3. **Save As** converts: a name ending in `.json` writes xcom2json JSON, any
   other name writes a game save.

## How saving works

- The first time you **Save** over a file, the original is copied to
  `<file>.bak`. Files are written to a temporary file and renamed into place,
  and the result is re-read before anything is written to disk.
- Game saves are rebuilt with the [go-xcomsave](https://github.com/scottvail/go-xcomsave)
  library; an
  unedited save is written back byte-for-byte identical.
- JSON files are written in xcom2json's exact layout with their original line
  endings, so only the edited lines change. Floats are written at full
  precision, so nothing is lost.
- Names are stored as Latin-1 when possible; names with other characters
  automatically become wide (UTF-16) strings.
- Fields missing from a soldier entry are shown disabled.

## Go library: `go-xcomsave`

Saves are read and written with
[go-xcomsave](https://github.com/scottvail/go-xcomsave), a pure-Go port of
[tracktwo/xcomsave](https://github.com/tracktwo/xcomsave) (the source of
`xcom2json`/`json2xcom`) that lives in its own repository. It also provides
Go versions of the `xcom2json` and `json2xcom` command-line tools, which
`make tools` builds alongside the editor.

Example saves (`example/`), used by the tests:

- `eu-source`, `ew-source`: Enemy Unknown and Enemy Within geoscape saves as
  the game wrote them.
- `eu-sample`, `ew-sample` (each also as `.json`): built from the sources with
  a known mix of soldier ranks, classes and load outs, interceptors,
  orders, projects, satellites and (Enemy Within) lab patients, as described
  in `internal/save/sample_test.go`. Rebuild them with `make samples`.

Command-line equivalents of the original tools:

```sh
go run github.com/scottvail/go-xcomsave/cmd/xcom2json [-o out.json] [-crlf|-lf] [-exact-floats] save62
go run github.com/scottvail/go-xcomsave/cmd/json2xcom [-o save62] save62.json
```

## Building

### All platforms with `make`

`make` builds the command-line tools and the editor for Windows (amd64),
Linux (amd64, arm64) and macOS (amd64, arm64) into
`build/dist/<os>/<arch>/bin`:

```sh
make                      # everything
make tools                # xcom2json / json2xcom only (pure Go, no Docker)
make editor               # the GUI editor only
make dist-linux_arm64     # tools + editor for one platform (<os>_<arch>)
make editor-windows_amd64 # editor for one platform
make test
make clean
```

The command-line tools are pure Go and cross-compile from any host. The
editor uses Fyne, which needs cgo, so each target needs a C toolchain:

| Target    | Built with | Requires |
|-----------|------------|----------|
| `darwin_*`  | host `clang -arch …` | a macOS host with the Xcode command line tools |
| `linux_*`   | Debian container for that architecture ([docker/builder.Dockerfile](docker/builder.Dockerfile)) | Docker |
| `windows_*` | the same container, using the MinGW-w64 cross compiler | Docker |

Linux builds for a different CPU architecture than the host run emulated and
are slow (10+ minutes the first time; Go caches are kept in `build/.cache`).

### Single native build

Requires Go 1.24+ and a C compiler:

- **macOS**: Xcode command line tools.
- **Linux**: `sudo apt-get install gcc libgl1-mesa-dev xorg-dev`
- **Windows**: a MinGW-w64 gcc (e.g. via MSYS2 or TDM-GCC) on `PATH`.

```sh
go build -o build/xcomsave-editor ./cmd/xcomsave-editor                               # macOS / Linux
go build -ldflags "-H=windowsgui" -o build/xcomsave-editor.exe ./cmd/xcomsave-editor  # Windows
```

You can also pass a file on the command line:
`xcomsave-editor path/to/save62`.

**Debug mode** (`xcomsave-editor -debug`, or Ctrl+Shift+D / Cmd+Shift+D
while running) shows the save property behind each field under it, e.g.
`m_kSoldier.iRank`, and the save entry names of soldiers and interceptors
(e.g. `XGStrategySoldier_3`).

### Working on go-xcomsave at the same time

To build against a local checkout of go-xcomsave instead of the published
version, clone it next to this repository and create a Go workspace (`go.work`
is ignored by git):

```sh
git clone https://github.com/scottvail/go-xcomsave ../go-xcomsave
go work init . ../go-xcomsave
```

The Docker builds ignore the workspace (`GOWORK=off`) and use the version in
`go.mod`.

The GitHub Actions workflow in `.github/workflows/build.yml` builds binaries
for all three platforms on every push.

## License

Copyright (C) 2026 Scott Vail.

This program is free software; you can redistribute it and/or modify it under
the terms of the GNU General Public License as published by the Free Software
Foundation; either version 2 of the License, or (at your option) any later
version. See [LICENSE](LICENSE) for the full text.

This program is distributed in the hope that it will be useful, but WITHOUT ANY
WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR A
PARTICULAR PURPOSE.

Saves are read and written with
[go-xcomsave](https://github.com/scottvail/go-xcomsave), a port of
[xcomsave](https://github.com/tracktwo/xcomsave) (Copyright (C) 2015 tracktwo,
GPL v2 or later) and [minilzo](http://www.oberhumer.com/opensource/lzo/)
(Copyright (C) 1996-2015 Markus F.X.J. Oberhumer, GPL v2 or later).
