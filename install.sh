#!/bin/sh
# Activate CLI installer — detects platform, downloads latest release, installs binary.
# Usage: curl -fsSL https://raw.githubusercontent.com/peregrine-digital/activate-framework/main/install.sh | sh
#
# Environment variables:
#   INSTALL_DIR   Override install directory (default: ~/.activate/bin)
#   VERSION       Install a specific version (default: latest)
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

# --- Download helpers ---

download() {
  url="$1"
  output="$2"
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL -o "$output" "$url"
  elif command -v wget >/dev/null 2>&1; then
    wget -qO "$output" "$url"
  else
    echo "Error: curl or wget is required." >&2
    exit 1
  fi
}

# --- Resolve version ---

resolve_version() {
  if [ -n "$VERSION" ]; then
    echo "$VERSION"
    return
  fi

  # Query GitHub API for latest release tag
  if command -v curl >/dev/null 2>&1; then
    TAG=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"//;s/".*//')
  elif command -v wget >/dev/null 2>&1; then
    TAG=$(wget -qO- "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"//;s/".*//')
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

# --- Main ---

main() {
  detect_platform

  TAG=$(resolve_version)
  VER="${TAG#v}"

  ASSET_NAME="${ARCHIVE_PREFIX}_${VER}_${OS_NAME}-${ARCH_NAME}.tar.gz"

  echo "Installing Activate CLI v${VER} (${OS_NAME}/${ARCH_NAME})..."

  TMPDIR=$(mktemp -d)
  trap 'rm -rf "$TMPDIR"' EXIT

  DOWNLOAD_BASE="https://github.com/${REPO}/releases/download/${TAG}"

  echo "Downloading ${ASSET_NAME}..."
  download "${DOWNLOAD_BASE}/${ASSET_NAME}" "${TMPDIR}/${ASSET_NAME}"

  # Try to download checksums
  download "${DOWNLOAD_BASE}/checksums.txt" "${TMPDIR}/checksums.txt" 2>/dev/null || true

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

  # Verify it's in PATH
  if ! echo "$PATH" | tr ':' '\n' | grep -qx "$INSTALL_DIR"; then
    echo ""
    echo "Add Activate to your PATH by adding this to your shell profile:"
    echo ""

    SHELL_NAME=$(basename "${SHELL:-/bin/sh}")
    case "$SHELL_NAME" in
      zsh)  RC="$HOME/.zshrc" ;;
      bash) RC="$HOME/.bashrc" ;;
      fish) RC="$HOME/.config/fish/config.fish" ;;
      *)    RC="your shell profile" ;;
    esac

    if [ "$SHELL_NAME" = "fish" ]; then
      echo "  fish_add_path $INSTALL_DIR"
    else
      echo "  export PATH=\"$INSTALL_DIR:\$PATH\""
    fi
    echo ""
    echo "Then restart your terminal or run:  source $RC"
  fi

  echo ""
  echo "✓ Activate CLI v${VER} installed to ${INSTALL_DIR}/${BINARY_NAME}"
}

main
