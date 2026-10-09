#!/bin/sh

set -eu

repository="${AUDIO_RECORD_REPOSITORY:-martynovb/audio_record}"
install_directory="${AUDIO_RECORD_INSTALL_DIR:-$HOME/.local/bin}"
version="${AUDIO_RECORD_VERSION:-latest}"

fail() {
  printf 'audio-record installer: %s\n' "$1" >&2
  exit 1
}

add_install_directory_to_path() {
  case ":$PATH:" in
    *":${install_directory}:"*) return ;;
  esac

  if [ "${AUDIO_RECORD_NO_PATH_UPDATE:-0}" = "1" ]; then
    printf 'Add %s to PATH manually.\n' "$install_directory" >&2
    return
  fi

  if [ -n "${AUDIO_RECORD_SHELL_PROFILE:-}" ]; then
    profile_file="$AUDIO_RECORD_SHELL_PROFILE"
  else
    case "${SHELL##*/}" in
      zsh) profile_file="$HOME/.zprofile" ;;
      bash) profile_file="$HOME/.bash_profile" ;;
      *)
        printf 'Add %s to PATH manually (unsupported shell: %s).\n' \
          "$install_directory" "${SHELL:-unknown}" >&2
        return
        ;;
    esac
  fi

  if [ "$install_directory" = "$HOME/.local/bin" ]; then
    path_line='export PATH="$HOME/.local/bin:$PATH"'
  else
    escaped_install_directory="$(
      printf '%s' "$install_directory" | sed 's/[\\"$`]/\\&/g'
    )"
    path_line="export PATH=\"${escaped_install_directory}:\$PATH\""
  fi

  if [ -f "$profile_file" ] && grep -Fqx "$path_line" "$profile_file"; then
    return
  fi

  mkdir -p "$(dirname "$profile_file")"
  printf '\n# Added by the audio-record installer\n%s\n' "$path_line" >> "$profile_file"
  printf 'Added %s to PATH in %s\n' "$install_directory" "$profile_file"
  printf 'Open a new terminal or run: . "%s"\n' "$profile_file"
}

command -v curl >/dev/null 2>&1 || fail "curl is required"
command -v tar >/dev/null 2>&1 || fail "tar is required"
command -v shasum >/dev/null 2>&1 || fail "shasum is required"

case "$(uname -s)" in
  Darwin) platform="darwin" ;;
  *) fail "use install.ps1 on Windows; this installer supports macOS only" ;;
esac

case "$(uname -m)" in
  arm64) architecture="arm64" ;;
  *) fail "this macOS release supports Apple Silicon only" ;;
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
add_install_directory_to_path
