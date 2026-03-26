#!/usr/bin/env bash

VERSION=$1
RELEASE=$2

VERSION=${VERSION//-/_}

DISTRO=$(. /etc/os-release && echo "${ID}${VERSION_ID}")
PKG_NAME="hbm"
PKG_DIR="/tmp/build/${PKG_NAME}_${VERSION}-${RELEASE}~${DISTRO}_amd64"

mkdir -p "${PKG_DIR}/DEBIAN"
mkdir -p "${PKG_DIR}/usr/sbin"
mkdir -p "${PKG_DIR}/etc/systemd/system"
mkdir -p "${PKG_DIR}/usr/share/bash-completion/completions"
mkdir -p "${PKG_DIR}/usr/share/man/man8"

# Install files from prebuild
cp /usr/local/src/hbm/hbm "${PKG_DIR}/usr/sbin/"
cp /usr/local/src/hbm/hbm.service "${PKG_DIR}/etc/systemd/system/"
cp /usr/local/src/hbm/hbm.socket "${PKG_DIR}/etc/systemd/system/"
cp /usr/local/src/hbm/shellcompletion/bash "${PKG_DIR}/usr/share/bash-completion/completions/hbm"
cp /usr/local/src/hbm/man/man8/*.8 "${PKG_DIR}/usr/share/man/man8/"

# Compress man pages
gzip -9 "${PKG_DIR}/usr/share/man/man8/"*.8

# Control file
cat > "${PKG_DIR}/DEBIAN/control" <<EOF
Package: ${PKG_NAME}
Version: ${VERSION}-${RELEASE}~${DISTRO}
Architecture: amd64
Maintainer: Kassisol <support@kassisol.com>
Description: Docker Engine Access Authorization Plugin
 HBM is an authorization plugin for docker commands.
Homepage: https://github.com/kassisol/hbm
License: GPL-3.0+
Section: admin
Priority: optional
EOF

# postinst - enable systemd units
cat > "${PKG_DIR}/DEBIAN/postinst" <<'EOF'
#!/bin/bash
if [ -d /run/systemd/system ]; then
	systemctl daemon-reload
	systemctl enable hbm.service hbm.socket
fi
EOF
chmod 755 "${PKG_DIR}/DEBIAN/postinst"

# prerm - stop and disable systemd units
cat > "${PKG_DIR}/DEBIAN/prerm" <<'EOF'
#!/bin/bash
if [ -d /run/systemd/system ]; then
	systemctl stop hbm.service hbm.socket
	systemctl disable hbm.service hbm.socket
fi
EOF
chmod 755 "${PKG_DIR}/DEBIAN/prerm"

dpkg-deb --build "${PKG_DIR}"

mkdir -p /tmp/dist
cp /tmp/build/*.deb /tmp/dist/
