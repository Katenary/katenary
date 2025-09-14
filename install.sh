#!/bin/sh

# Install the latest version of the Katenary detecting the right OS and architecture.
# Can be launched with the following command:
# sh <(curl -sSL https://raw.githubusercontent.com/Katenary/katenary/master/install.sh)

# Detect the OS and architecture
OS=$(uname)
ARCH=$(uname -m)

for c in curl grep cut tr; do
	if ! command -v $c >/dev/null 2>&1; then
		echo "Error: $c is not installed"
		exit 1
	fi
done

# Detect the home directory "bin" directory, it is commonly:
# - $HOME/.local/bin
# - $HOME/.bin
# - $HOME/bin
COMMON_INSTALL_PATHS="$HOME/.local/bin $HOME/.bin $HOME/bin"

INSTALL_PATH=""
for p in $COMMON_INSTALL_PATHS; do
	if [ -d $p ]; then
		INSTALL_PATH=$p
		break
	fi
done

# check if the user has write access to the INSTALL_PATH
if [ -z "$INSTALL_PATH" ]; then
	INSTALL_PATH="/usr/local/bin"
	if [ ! -w $INSTALL_PATH ]; then
		echo "You don't have write access to $INSTALL_PATH"
		echo "Please, run with sudo or install locally"
		exit 1
	fi
fi

# ensure that $INSTALL_PATH is in the PATH
if ! echo "$PATH" | grep -q "$INSTALL_PATH"; then
	echo "Sorry, ${INSTALL_PATH} is not in the PATH"
	echo "Please, add it to your PATH in your shell configuration file"
	echo "then restart your shell and run this script again"
	exit 1
fi

# Where to download the binary
TAG=$(curl -sLf https://repo.katenary.io/api/v1/repos/katenary/katenary/releases/latest 2>/dev/null | grep -Po '"tag_name":\s*"[^"]*"' | cut -d ":" -f2 | tr -d '"')

# use the right names for the OS and architecture
if [ $ARCH = "x86_64" ]; then
	ARCH="amd64"
fi

BIN_URL="https://repo.katenary.io/api/packages/Katenary/generic/katenary/$TAG/katenary-$OS-$ARCH"

echo
echo "Downloading $BIN_URL"

T=$(mktemp -u)
curl -sLf -# $BIN_URL -o $T 2>/dev/null || (echo -e "Failed to download katenary version $TAG.\n\nPlease open an issue and explain the problem, following the link:\nhttps://repo.katenary.io/Katenary/katenary/issues/new?title=[install.sh]%20Install%20$TAG%20failed" && rm -f $T && exit 1)

mv "$T" "${INSTALL_PATH}/katenary"
chmod +x "${INSTALL_PATH}/katenary"
echo
echo "Installed to $INSTALL_PATH/katenary"
echo "Installation complete! Run 'katenary help' to get started."
