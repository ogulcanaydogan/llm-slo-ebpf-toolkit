#!/usr/bin/env bash
set -euo pipefail

AUTO_INSTALL="${AUTO_INSTALL:-0}"
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"

missing=()
for bin in go docker kubectl kind helm; do
  if ! command -v "$bin" >/dev/null 2>&1; then
    missing+=("$bin")
  fi
done

if [[ "$OS" == "linux" ]]; then
  for bin in bpftool clang; do
    if ! command -v "$bin" >/dev/null 2>&1; then
      missing+=("$bin")
    fi
  done
fi

if [[ ${#missing[@]} -eq 0 ]]; then
  echo "all required tools are installed"
  exit 0
fi

echo "missing tools: ${missing[*]}"

if [[ "$AUTO_INSTALL" != "1" ]]; then
  echo
  echo "set AUTO_INSTALL=1 to run best-effort install commands."
  echo "recommended manual installs:"
  if [[ "$OS" == "darwin" ]]; then
    echo "  brew install kubectl kind helm llvm"
  elif [[ "$OS" == "linux" ]]; then
    echo "  sudo apt-get update"
    echo "  sudo apt-get install -y bpftool clang llvm"
    echo "  # install kind/helm/kubectl from vendor instructions if not in apt repo"
  fi
  exit 1
fi

if [[ "$OS" == "darwin" ]]; then
  if ! command -v brew >/dev/null 2>&1; then
    echo "brew is required for AUTO_INSTALL on macOS" >&2
    exit 1
  fi
  brew install kubectl kind helm llvm
  echo "macOS tool bootstrap complete"
  exit 0
fi

if [[ "$OS" == "linux" ]]; then
  if ! command -v apt-get >/dev/null 2>&1; then
    echo "automatic Linux install currently supports apt-get only" >&2
    exit 1
  fi
  if [[ "$(id -u)" -ne 0 ]]; then
    echo "run AUTO_INSTALL=1 as root for apt installs" >&2
    exit 1
  fi
  apt-get update
  apt-get install -y bpftool clang llvm kubectl helm
  if ! command -v kind >/dev/null 2>&1; then
    curl -Lo /usr/local/bin/kind https://kind.sigs.k8s.io/dl/v0.24.0/kind-linux-amd64
    chmod +x /usr/local/bin/kind
  fi
  echo "linux tool bootstrap complete"
  exit 0
fi

echo "unsupported OS: $OS" >&2
exit 1
