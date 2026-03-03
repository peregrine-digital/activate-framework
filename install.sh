#!/bin/sh
# Activate CLI installer — detects platform, downloads latest release, installs binary.
# Usage: curl -fsSL https://raw.githubusercontent.com/peregrine-digital/activate-framework/main/install.sh | sh
#
# Environment variables:
#   INSTALL_DIR   Override install directory (default: ~/.activate/bin)
#   VERSION       Install a specific version (default: latest)
#   GITHUB_TOKEN  GitHub token for private repos (optional)
set -e

REPO="peregrine-digital/activate-framework"
BINARY_NAME="activate"
ARCHIVE_PREFIX="activate-framework"

# --- Platform detection ---

detect_platform() {
  OS=$(uname -s)
  ARCH=$(uname -m)

  case "$OS" in
    Darwin) OS_NAME="darwin" ;;
    Linux)  OS_NAME="linux" ;;
    *)
      echo "Error: Unsupported operating system: $OS" >&2
      echo "Activate supports macOS and Linux." >&2
      exit 1
      ;;
  esac

  case "$ARCH" in
    arm64|aarch64) ARCH_NAME="arm64" ;;
    x86_64|amd64)  ARCH_NAME="amd64" ;;
    *)
      echo "Error: Unsupported architecture: $ARCH" >&2
      echo "Activate supports arm64 and x86_64 (amd64)." >&2
      exit 1
      ;;
  esac

  # Detect Rosetta 2 on macOS — prefer native arm64 binary
  if [ "$OS_NAME" = "darwin" ] && [ "$ARCH_NAME" = "amd64" ]; then
    if sysctl -n sysctl.proc_translated 2>/dev/null | grep -q 1; then
      echo "Detected Rosetta 2 — installing native arm64 binary instead."
      ARCH_NAME="arm64"
    fi
  fi
}

# --- Auth header ---

auth_header() {
  if [ -n "$GITHUB_TOKEN" ]; then
    echo "Authorization: token $GITHUB_TOKEN"
  fi
}

# --- Download helpers ---

download() {
  url="$1"
  output="$2"
  if command -v curl >/dev/null 2>&1; then
    if [ -n "$GITHUB_TOKEN" ]; then
      curl -fsSL -H "Authorization: token $GITHUB_TOKEN" -H "Accept: application/octet-stream" -o "$output" "$url"
    else
      curl -fsSL -o "$output" "$url"
    fi
  elif command -v wget >/dev/null 2>&1; then
    if [ -n "$GITHUB_TOKEN" ]; then
      wget --header="Authorization: token $GITHUB_TOKEN" --header="Accept: application/octet-stream" -qO "$output" "$url"
    else
      wget -qO "$output" "$url"
    fi
  else
    echo "Error: curl or wget is required." >&2
    exit 1
  fi
}

# Resolve the API download URL for a release asset by name.
# For private repos, assets must be downloaded via the API endpoint.
resolve_asset_url() {
  asset_name="$1"
  release_tag="$2"

  if [ -n "$GITHUB_TOKEN" ]; then
    # Use API to get the asset ID, then download via API endpoint
    if command -v curl >/dev/null 2>&1; then
      asset_url=$(curl -fsSL -H "Authorization: token $GITHUB_TOKEN" \
        "https://api.github.com/repos/${REPO}/releases/tags/${release_tag}" \
        | grep -B3 "\"name\": \"${asset_name}\"" | grep '"url"' | head -1 \
        | sed 's/.*"url": *"//;s/".*//')
    elif command -v wget >/dev/null 2>&1; then
      asset_url=$(wget --header="Authorization: token $GITHUB_TOKEN" -qO- \
        "https://api.github.com/repos/${REPO}/releases/tags/${release_tag}" \
        | grep -B3 "\"name\": \"${asset_name}\"" | grep '"url"' | head -1 \
        | sed 's/.*"url": *"//;s/".*//')
    fi

    if [ -n "$asset_url" ]; then
      echo "$asset_url"
      return
    fi
  fi

  # Fallback to direct URL (works for public repos)
  echo "https://github.com/${REPO}/releases/download/${release_tag}/${asset_name}"
}

# --- Resolve version ---

resolve_version() {
  if [ -n "$VERSION" ]; then
    echo "$VERSION"
    return
  fi

  # Query GitHub API for latest release tag (uses /releases[0] to include pre-releases)
  if command -v curl >/dev/null 2>&1; then
    if [ -n "$GITHUB_TOKEN" ]; then
      TAG=$(curl -fsSL -H "Authorization: token $GITHUB_TOKEN" "https://api.github.com/repos/${REPO}/releases?per_page=1" | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"//;s/".*//')
    else
      TAG=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases?per_page=1" | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"//;s/".*//')
    fi
  elif command -v wget >/dev/null 2>&1; then
    if [ -n "$GITHUB_TOKEN" ]; then
      TAG=$(wget --header="Authorization: token $GITHUB_TOKEN" -qO- "https://api.github.com/repos/${REPO}/releases?per_page=1" | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"//;s/".*//')
    else
      TAG=$(wget -qO- "https://api.github.com/repos/${REPO}/releases?per_page=1" | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"//;s/".*//')
    fi
  fi

  if [ -z "$TAG" ]; then
    echo "Error: Could not determine latest version." >&2
    exit 1
  fi

  echo "$TAG"
}

# --- Checksum verification ---

verify_checksum() {
  binary_path="$1"
  asset_name="$2"
  checksums_path="$3"

  if [ ! -f "$checksums_path" ]; then
    echo "Warning: No checksums file found, skipping verification."
    return 0
  fi

  expected=$(grep "$asset_name" "$checksums_path" | awk '{print $1}')
  if [ -z "$expected" ]; then
    echo "Warning: No checksum found for $asset_name, skipping verification."
    return 0
  fi

  if command -v shasum >/dev/null 2>&1; then
    actual=$(shasum -a 256 "$binary_path" | awk '{print $1}')
  elif command -v sha256sum >/dev/null 2>&1; then
    actual=$(sha256sum "$binary_path" | awk '{print $1}')
  else
    echo "Warning: Cannot verify checksum (no shasum or sha256sum found)."
    return 0
  fi

  if [ "$actual" != "$expected" ]; then
    echo "Error: Checksum verification failed!" >&2
    echo "  Expected: $expected" >&2
    echo "  Got:      $actual" >&2
    exit 1
  fi

  echo "✓ Checksum verified."
}

# --- PATH management ---
# Writes PATH entry to shell profile files, similar to rustup/nvm.
# Idempotent: uses a marker comment to avoid duplicate entries.

ACTIVATE_MARKER="# Added by Activate CLI installer"

# Append a PATH entry to a single profile file (idempotent).
_add_line() {
  profile="$1"
  line="$2"
  [ -z "$profile" ] && return

  # Already present
  if [ -f "$profile" ] && grep -qF "$ACTIVATE_MARKER" "$profile"; then
    return
  fi

  # Create parent dirs if needed (e.g. fish config)
  mkdir -p "$(dirname "$profile")"

  printf '\n%s\n%s\n' "$ACTIVATE_MARKER" "$line" >> "$profile"
  echo "  ✓ Updated $profile"
}

add_to_path() {
  dir="$1"

  # If already in PATH, nothing to do
  if echo "$PATH" | tr ':' '\n' | grep -qx "$dir"; then
    return
  fi

  SHELL_NAME=$(basename "${SHELL:-/bin/sh}")
  EXPORT_LINE="export PATH=\"${dir}:\$PATH\""
  modified=false

  case "$SHELL_NAME" in
    zsh)
      # .zshenv is sourced by ALL zsh sessions (login, interactive, scripts)
      _add_line "$HOME/.zshenv" "$EXPORT_LINE"
      modified=true
      ;;
    bash)
      # .bashrc for interactive shells
      _add_line "$HOME/.bashrc" "$EXPORT_LINE"
      # .bash_profile for login shells (macOS Terminal.app uses login shells)
      if [ -f "$HOME/.bash_profile" ]; then
        _add_line "$HOME/.bash_profile" "$EXPORT_LINE"
      fi
      modified=true
      ;;
    fish)
      _add_line "$HOME/.config/fish/config.fish" "fish_add_path $dir"
      modified=true
      ;;
  esac

  # POSIX fallback — .profile is sourced by sh, dash, and bash login shells
  # when .bash_profile doesn't exist. Always write it for portability.
  if [ "$SHELL_NAME" != "fish" ]; then
    _add_line "$HOME/.profile" "$EXPORT_LINE"
    modified=true
  fi

  if [ "$modified" = true ]; then
    echo "✓ PATH updated. Restart your terminal to apply."
  else
    echo ""
    echo "Add Activate to your PATH manually:"
    echo "  $EXPORT_LINE"
  fi

  # Make available for rest of this script
  export PATH="$dir:$PATH"
}

# --- Main ---

main() {
  detect_platform

  TAG=$(resolve_version)
  VER="${TAG#v}"

  ASSET_NAME="${ARCHIVE_PREFIX}_${VER}_${OS_NAME}-${ARCH_NAME}.tar.gz"

  echo "Installing Activate CLI v${VER} (${OS_NAME}/${ARCH_NAME})..."

  TMPDIR=$(mktemp -d)
  trap 'rm -rf "$TMPDIR"' EXIT

  echo "Downloading ${ASSET_NAME}..."
  ASSET_URL=$(resolve_asset_url "$ASSET_NAME" "$TAG")
  download "$ASSET_URL" "${TMPDIR}/${ASSET_NAME}"

  # Try to download checksums
  CHECKSUMS_URL=$(resolve_asset_url "checksums.txt" "$TAG")
  download "$CHECKSUMS_URL" "${TMPDIR}/checksums.txt" 2>/dev/null || true

  verify_checksum "${TMPDIR}/${ASSET_NAME}" "$ASSET_NAME" "${TMPDIR}/checksums.txt"

  # Extract
  tar xzf "${TMPDIR}/${ASSET_NAME}" -C "$TMPDIR"
  chmod +x "${TMPDIR}/${BINARY_NAME}"

  # Determine install directory
  if [ -z "$INSTALL_DIR" ]; then
    INSTALL_DIR="$HOME/.activate/bin"
  fi

  mkdir -p "$INSTALL_DIR"
  mv "${TMPDIR}/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"

  # Add to PATH if not already there
  add_to_path "$INSTALL_DIR"

  echo ""
  echo "✓ Activate CLI v${VER} installed to ${INSTALL_DIR}/${BINARY_NAME}"
}

main
