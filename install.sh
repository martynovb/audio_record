#!/bin/sh

set -eu

repository="${AUDIO_RECORD_REPOSITORY:-martynovb/audio_record}"
install_directory="${AUDIO_RECORD_INSTALL_DIR:-$HOME/.local/bin}"
version="${AUDIO_RECORD_VERSION:-latest}"

fail() {
  printf 'audio-record installer: %s\n' "$1" >&2
  exit 1
}

command -v curl >/dev/null 2>&1 || fail "curl is required"
command -v tar >/dev/null 2>&1 || fail "tar is required"
command -v shasum >/dev/null 2>&1 || fail "shasum is required"

case "$(uname -s)" in
  Darwin) platform="darwin" ;;
  *) fail "this release currently supports macOS only; Windows support is not implemented yet" ;;
esac

case "$(uname -m)" in
  arm64) architecture="arm64" ;;
  x86_64) architecture="amd64" ;;
  *) fail "unsupported architecture: $(uname -m)" ;;
esac

asset="audio-record-${platform}-${architecture}.tar.gz"
if [ -n "${AUDIO_RECORD_DOWNLOAD_BASE:-}" ]; then
  download_base="${AUDIO_RECORD_DOWNLOAD_BASE%/}"
elif [ "$version" = "latest" ]; then
  download_base="https://github.com/${repository}/releases/latest/download"
else
  download_base="https://github.com/${repository}/releases/download/${version}"
fi

temporary_directory="$(mktemp -d "${TMPDIR:-/tmp}/audio-record-install.XXXXXX")"
trap 'rm -rf "$temporary_directory"' EXIT HUP INT TERM

printf 'Downloading %s...\n' "$asset"
curl --fail --location --silent --show-error \
  "${download_base}/${asset}" \
  --output "${temporary_directory}/${asset}"
curl --fail --location --silent --show-error \
  "${download_base}/${asset}.sha256" \
  --output "${temporary_directory}/${asset}.sha256"

(
  cd "$temporary_directory"
  shasum -a 256 -c "${asset}.sha256"
  tar -xzf "$asset"
)

mkdir -p "$install_directory"
install -m 0755 \
  "${temporary_directory}/audio-record" \
  "${install_directory}/audio-record"

printf 'Installed audio-record to %s/audio-record\n' "$install_directory"
case ":$PATH:" in
  *":${install_directory}:"*) ;;
  *)
    printf '%s\n' "Add this directory to PATH:" >&2
    printf '  export PATH="%s:$PATH"\n' "$install_directory" >&2
    ;;
esac
