#!/usr/bin/env bash
# Builds the Linux program as an AppImage: one file that runs on its own,
# with the game catalog inside.
#
#	tools/appimage.sh                  # version from git describe
#	tools/appimage.sh v0.1.0-beta.3    # the version tag
#
# Output: dist/RagnarokOfBuilding-<version>-x86_64.AppImage. Needs Go and the
# Gio build libraries (docs/DESENVOLVIMENTO.md); appimagetool is downloaded
# once into dist/tools. The program keeps its files (settings, saved builds,
# icons, log) in ~/.local/share/ragnarok-of-building.
set -euo pipefail
cd "$(dirname "$0")/.."

ver="${1:-$(git describe --tags --always 2>/dev/null || echo dev)}"
repo="$(git remote get-url origin 2>/dev/null | sed -E 's#.*github.com[:/]##; s#\.git$##')"
repo="${repo:-Loonguinho/ragnarok-of-building}"

tool_ver=1.9.1
tool=dist/tools/appimagetool-$tool_ver-x86_64.AppImage
if [ ! -x "$tool" ]; then
	mkdir -p dist/tools
	echo "Downloading appimagetool $tool_ver…"
	curl -fsSL -o "$tool.part" \
		"https://github.com/AppImage/appimagetool/releases/download/$tool_ver/appimagetool-x86_64.AppImage"
	chmod +x "$tool.part"
	mv "$tool.part" "$tool"
fi

app=dist/RagnarokOfBuilding.AppDir
rm -rf "$app"
mkdir -p "$app/usr/bin" "$app/usr/share/ragnarok-of-building"

echo "Building $ver…"
CGO_ENABLED=1 go build -trimpath \
	-ldflags "-s -w -X main.version=$ver -X main.repo=$repo" \
	-o "$app/usr/bin/RagnarokOfBuilding" ./cmd/app
cp data/gen/catalog.json.gz "$app/usr/share/ragnarok-of-building/"
cp cmd/app/icon.png "$app/ragnarok-of-building.png"
ln -s ragnarok-of-building.png "$app/.DirIcon"

cat >"$app/ragnarok-of-building.desktop" <<'EOF'
[Desktop Entry]
Type=Application
Name=Ragnarok of Building
Comment=Build and damage simulator for Ragnarok Online LATAM
Exec=RagnarokOfBuilding
Icon=ragnarok-of-building
Categories=Game;Utility;
Terminal=false
EOF

cat >"$app/AppRun" <<'EOF'
#!/bin/sh
# The keyboard's compose tables: a libxkbcommon built into /usr/local looks
# for them there and the window does not open.
if [ -z "${XLOCALEDIR:-}" ] && [ -d /usr/share/X11/locale ]; then
	export XLOCALEDIR=/usr/share/X11/locale
fi
exec "${APPDIR:-$(dirname "$(readlink -f "$0")")}/usr/bin/RagnarokOfBuilding" "$@"
EOF
chmod +x "$app/AppRun"

out="dist/RagnarokOfBuilding-$ver-x86_64.AppImage"
ARCH=x86_64 "$tool" --no-appstream "$app" "$out"
echo "Done: $out"
