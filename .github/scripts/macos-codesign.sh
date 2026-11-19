#!/usr/bin/env bash
# SPDX-License-Identifier: GPL-2.0-or-later
# Copyright (C) 2026 Scott Vail
#
# macOS code signing and notarization for GitHub Actions (macOS runners).
#
#   macos-codesign.sh setup                  import the Developer ID certificate
#   macos-codesign.sh sign [codesign opts] PATH...
#                                            sign PATHs with Developer ID, or
#                                            ad hoc if there is no certificate
#   macos-codesign.sh notarize PATH          notarize a .zip, .dmg, .pkg or .app,
#                                            and staple the ticket where possible
#
# Secrets, passed as environment variables:
#   MACOS_CERT_P12       Developer ID Application certificate + key, .p12, base64
#   MACOS_CERT_PASSWORD  the .p12 password
#   NOTARY_KEY_P8        App Store Connect API key (.p8 file contents)
#   NOTARY_KEY_ID        its Key ID
#   NOTARY_ISSUER_ID     its Issuer ID
#
# Without MACOS_CERT_P12 everything is ad hoc signed and notarization is
# skipped (with a warning), so builds still work without the secrets; the
# result just triggers Gatekeeper's "unverified developer" prompt.

set -euo pipefail

kc="$RUNNER_TEMP/signing.keychain-db"

identity() {
	security find-identity -v -p codesigning "$kc" |
		awk '/"Developer ID Application/ { print $2; exit }'
}

have_cert() { [ -f "$kc" ]; }

case "${1:-}" in
setup)
	if [ -z "${MACOS_CERT_P12:-}" ]; then
		echo "::warning::MACOS_CERT_P12 is not set: macOS builds will be ad hoc signed and not notarized"
		exit 0
	fi
	pass=$(openssl rand -hex 24)
	security create-keychain -p "$pass" "$kc"
	security set-keychain-settings -lut 21600 "$kc"
	security unlock-keychain -p "$pass" "$kc"
	p12="$RUNNER_TEMP/cert.p12"
	printf '%s' "$MACOS_CERT_P12" | base64 --decode >"$p12"
	security import "$p12" -k "$kc" -P "${MACOS_CERT_PASSWORD:-}" -T /usr/bin/codesign
	rm -f "$p12"
	security set-key-partition-list -S apple-tool:,apple:,codesign: -s -k "$pass" "$kc" >/dev/null
	# shellcheck disable=SC2046
	security list-keychains -d user -s "$kc" $(security list-keychains -d user | tr -d '"')
	if [ -z "$(identity)" ]; then
		echo "::error::MACOS_CERT_P12 has no \"Developer ID Application\" identity (an Apple Distribution certificate won't work)"
		exit 1
	fi
	security find-identity -v -p codesigning "$kc"
	;;

sign)
	shift
	if have_cert; then
		codesign --force --timestamp --sign "$(identity)" "$@"
	else
		codesign --force --sign - "$@"
	fi
	;;

notarize)
	path=$2
	if ! have_cert || [ -z "${NOTARY_KEY_P8:-}" ]; then
		echo "::warning::not notarizing $(basename "$path"): the signing certificate or notary key is not set"
		exit 0
	fi
	key="$RUNNER_TEMP/notary.p8"
	printf '%s\n' "$NOTARY_KEY_P8" >"$key"
	auth=(--key "$key" --key-id "$NOTARY_KEY_ID" --issuer "$NOTARY_ISSUER_ID")

	submit=$path
	if [ -d "$path" ]; then # an .app bundle: notarytool takes it zipped
		submit="$RUNNER_TEMP/$(basename "$path").zip"
		ditto -c -k --keepParent "$path" "$submit"
	fi

	result=$(xcrun notarytool submit "$submit" "${auth[@]}" --wait --output-format json) || true
	echo "$result"
	id=$(jq -r '.id // empty' <<<"$result")
	status=$(jq -r '.status // empty' <<<"$result")
	if [ "$status" != Accepted ]; then
		echo "::error::notarization of $(basename "$path") failed: ${status:-no result}"
		[ -n "$id" ] && xcrun notarytool log "$id" "${auth[@]}" || true
		rm -f "$key"
		exit 1
	fi
	rm -f "$key"
	[ "$submit" != "$path" ] && rm -f "$submit"

	# Bare binaries and zips can't hold a ticket; Gatekeeper looks those up
	# online instead.
	case "$path" in
	*.app | *.dmg | *.pkg) xcrun stapler staple "$path" ;;
	esac
	;;

*)
	sed -n '5,13p' "$0" >&2
	exit 2
	;;
esac
