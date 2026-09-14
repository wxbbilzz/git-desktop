#!/bin/bash
#
# Bingit —— 按《UOS 应用打包规范》构建 deb 包
#
# 规范要点（对应文档章节）：
#   §1 appid 用倒置域名，全小写           -> org.bingit.app
#   §2 所有文件装在 /opt/apps/${appid}/   -> 见 PKGROOT
#   §3 根目录必须含 entries/ files/ info
#   §4 info 为 JSON，version 与 deb 一致
#   §5 应用数据只写 $XDG_*_HOME/${appid}
#   §6 用 dpkg-deb --build --root-owner-group，且不使用 deb 钩子
#
set -euo pipefail

APPID="org.bingit.app"
ARCH="amd64"
NAME="Bingit"
NAME_ZH="冰冰 Git"

HERE="$(cd "$(dirname "$0")/.." && pwd)"
OUT="$HERE/build/deb"

# ---------- 版本号：唯一来源是 frontend/src/version.ts ----------
# 「关于」对话框显示的就是这个值，这里再读一遍，两者永远不会对不上。
# 读不到或格式不对就直接失败，绝不拿一个空版本号去打 deb。
VERSION_FILE="$HERE/frontend/src/version.ts"
VERSION="$(sed -n 's/.*APP_VERSION[[:space:]]*=[[:space:]]*"\([^"]*\)".*/\1/p' "$VERSION_FILE" | head -1)"

if [[ ! "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "无法从 $VERSION_FILE 读出合法版本号（应为 MAJOR.MINOR.PATCH.BUILD），实际读到：「$VERSION」" >&2
  exit 1
fi

PKGROOT="$OUT/${APPID}_${VERSION}_${ARCH}"

# 清理上一次的构建产物
rm -rf "$PKGROOT"
mkdir -p "$PKGROOT/DEBIAN"

# ---------- 先编译 ----------
# 这里必须让编译失败直接终止脚本。
# 老写法是 `wails build 2>&1 | grep -E "Built|error" || true`，
# 管道把退出码吃掉了：一旦编译失败，下面的 [ -f "$BIN" ] 会命中
# 上一次残留的旧二进制，于是打出来的 deb 里装的是过期的程序，
# 而且脚本还会打印「打包完成」。所以改成：先删旧产物，再让 wails build
# 自己决定成败（set -e 会让它失败即退出）。
echo "==> 版本 $VERSION（读自 frontend/src/version.ts）"
echo "==> 编译"
cd "$HERE"
BIN="$HERE/build/bin/bingit"
rm -f "$BIN"
wails build

[ -f "$BIN" ] || { echo "编译失败：找不到产物 $BIN"; exit 1; }

# ---------- §3 应用目录结构 ----------
APPDIR="$PKGROOT/opt/apps/$APPID"
mkdir -p "$APPDIR/entries/applications"
mkdir -p "$APPDIR/entries/icons/hicolor/256x256/apps"
mkdir -p "$APPDIR/entries/icons/hicolor/scalable/apps"
mkdir -p "$APPDIR/files/bin"

# 主程序
install -m 755 "$BIN" "$APPDIR/files/bin/bingit"

# ---------- §5 启动脚本：把数据限制在自己的目录 ----------
cat > "$APPDIR/files/bin/bingit-launch" <<'LAUNCH'
#!/bin/sh
# Bingit 启动脚本。
#
# 按 UOS 规范 §5：应用数据只能写自己的 XDG 目录，
# 不能直接往 $HOME 里扔东西。这里把 lazygit 核心读配置用的
# CONFIG_DIR 指到 $XDG_CONFIG_HOME/${appid}，其余目录也一并建好。
APPID=org.bingit.app

: "${XDG_CONFIG_HOME:=$HOME/.config}"
: "${XDG_DATA_HOME:=$HOME/.local/share}"
: "${XDG_CACHE_HOME:=$HOME/.cache}"

export XDG_CONFIG_HOME XDG_DATA_HOME XDG_CACHE_HOME
export CONFIG_DIR="$XDG_CONFIG_HOME/$APPID"

mkdir -p "$CONFIG_DIR" \
         "$XDG_DATA_HOME/$APPID" \
         "$XDG_CACHE_HOME/$APPID" 2>/dev/null || true

exec "/opt/apps/$APPID/files/bin/bingit" "$@"
LAUNCH
chmod 755 "$APPDIR/files/bin/bingit-launch"

# ---------- §3 图标 ----------
install -m 644 "$HERE/assets/bingit-icon.png" \
  "$APPDIR/entries/icons/hicolor/256x256/apps/$APPID.png"
install -m 644 "$HERE/assets/bingit-icon.svg" \
  "$APPDIR/entries/icons/hicolor/scalable/apps/$APPID.svg"

# ---------- §3 desktop 文件 ----------
cat > "$APPDIR/entries/applications/$APPID.desktop" <<DESKTOP
[Desktop Entry]
Type=Application
Version=1.0
Name=$NAME
Name[zh_CN]=$NAME_ZH
Comment=A git client with a friendly interface
Comment[zh_CN]=界面友好的 Git 客户端
Exec=/opt/apps/$APPID/files/bin/bingit-launch
Icon=$APPID
Terminal=false
StartupNotify=true
StartupWMClass=$APPID
Categories=Development;RevisionControl;
Keywords=git;bingit;lazygit;vcs;
Keywords[zh_CN]=git;冰冰;版本控制;仓库;
DESKTOP

# ---------- §4 info 文件 ----------
cat > "$APPDIR/info" <<INFO
{
    "appid": "$APPID",
    "name": "$NAME",
    "version": "$VERSION",
    "arch": ["$ARCH"],
    "permissions": {
        "autostart": false,
        "notification": false,
        "trayicon": false,
        "clipboard": false,
        "account": false,
        "bluetooth": false,
        "camera": false,
        "audio_record": false,
        "installed_apps": false
    },
    "support-plugins": [],
    "plugins": []
}
INFO

# ---------- §6 DEBIAN/control ----------
# 注意：规范明确不允许用 postinst/postrm 等钩子改系统文件，
# 所以这里只有 control，没有任何 maintainer script。
INSTALLED_KB=$(du -sk "$APPDIR" | cut -f1)
cat > "$PKGROOT/DEBIAN/control" <<CONTROL
Package: $APPID
Version: $VERSION
Section: devel
Priority: optional
Architecture: $ARCH
Installed-Size: $INSTALLED_KB
Maintainer: 小冰冰 <bingit@example.com>
Depends: libwebkit2gtk-4.1-0 | libwebkit2gtk-4.0-0, libgtk-3-0
Description: $NAME - a git client with a friendly interface
 Bingit 是一个桌面版 Git 客户端，复用 lazygit 的 git 命令层，
 界面完全重写：文件树、行级暂存、提交图、冲突解决、音效等。
CONTROL

# ---------- §6 构建 ----------
DEB="$OUT/${APPID}_${VERSION}_${ARCH}.deb"
mkdir -p "$OUT"
dpkg-deb --build --root-owner-group "$PKGROOT" "$DEB" >/dev/null

echo
echo "==> 打包完成: $DEB"
ls -lh "$DEB"
