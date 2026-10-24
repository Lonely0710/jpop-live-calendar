#!/usr/bin/env python3
"""Give the macOS disk image of the app a designed Finder window.

`mygo build` writes a .dmg whose window shows the app next to a link to
/Applications, over a plain white background (see cmd/mygo/dsstore.go in
MyGo). This script replaces that background with assets/dmg/background.png
(the same paper, grain and type as assets/readme/hero.svg) and rewrites the
window's .DS_Store so the icons sit on the empty zones of the artwork.

How it works: the compressed image is converted to a writable one, mounted,
given .background/bg.tiff, then detached and converted back. The picture is
referenced the only way Finder understands, as an alias record stored in
.DS_Store (key "backgroundImageAlias"), which the Alias Manager builds here
through CoreServices: that is the record Finder itself writes, so there is
no alias encoder to maintain, and no third-party tool to install.

usage: scripts/dmg-layout.py build/app.dmg [--background png] [--background-2x png]

Only macOS has the tools this needs (hdiutil, tiffutil); on other systems
it exits with an error and the image stays as mygo built it.
"""

import argparse
import ctypes
import ctypes.util
import os
import plistlib
import shutil
import struct
import subprocess
import sys
import tempfile
import time

# Artwork zones, in points from the top-left corner of the window content,
# matching assets/dmg/background.svg (680x420).
DEFAULT_WINDOW = (680, 420)
DEFAULT_ICON_SIZE = 128
DEFAULT_APP = (185, 200)
DEFAULT_APPLICATIONS = (495, 200)
DEFAULT_APP_NAME = "Live Calendar.app"

# Finder places the window this far from the bottom-left corner of the screen.
WINDOW_POSITION = (200, 400)

BACKGROUND_DIR = ".background"
BACKGROUND_FILE = "bg.tiff"


class LayoutError(Exception):
    pass


def run(*args):
    try:
        return subprocess.run(args, check=True, capture_output=True, text=True).stdout
    except subprocess.CalledProcessError as err:
        raise LayoutError(
            "%s failed (%d): %s" % (args[0], err.returncode, (err.stderr or err.stdout).strip())
        ) from err
    except FileNotFoundError as err:
        raise LayoutError("%s not found; this needs macOS" % args[0]) from err


# --- alias record ---------------------------------------------------------
#
# The Alias Manager hands out a handle: a pointer to a pointer to the record,
# whose first two bytes of the record body hold its size, big-endian, at
# offset 4. The bytes of that record are what .DS_Store stores.


def core_services():
    path = ctypes.util.find_library("CoreServices")
    if not path:
        raise LayoutError("CoreServices is not available; this needs macOS")
    api = ctypes.CDLL(path)
    api.FSNewAliasFromPath.argtypes = [
        ctypes.c_char_p,
        ctypes.c_char_p,
        ctypes.POINTER(ctypes.c_ubyte),
        ctypes.POINTER(ctypes.c_void_p),
    ]
    api.FSNewAliasFromPath.restype = ctypes.c_int32
    api.PtrToHand.argtypes = [ctypes.c_char_p, ctypes.POINTER(ctypes.c_void_p), ctypes.c_long]
    api.PtrToHand.restype = ctypes.c_int16
    api.FSResolveAlias.argtypes = [
        ctypes.c_void_p,
        ctypes.c_void_p,
        ctypes.c_void_p,
        ctypes.POINTER(ctypes.c_ubyte),
    ]
    api.FSResolveAlias.restype = ctypes.c_int16
    api.FSRefMakePath.argtypes = [ctypes.c_void_p, ctypes.c_char_p, ctypes.c_uint32]
    api.FSRefMakePath.restype = ctypes.c_int16
    return api


def alias_record(from_dir, target_path):
    api = core_services()
    is_dir = ctypes.c_ubyte(0)
    handle = ctypes.c_void_p()
    status = api.FSNewAliasFromPath(
        os.fsencode(from_dir), os.fsencode(target_path), ctypes.byref(is_dir), ctypes.byref(handle)
    )
    if status != 0:
        raise LayoutError("FSNewAliasFromPath(%s) failed: %d" % (target_path, status))
    try:
        record = ctypes.c_void_p.from_address(handle.value).value
        size = struct.unpack(">H", ctypes.string_at(record + 4, 2))[0]
        record = ctypes.string_at(record, 2 + size)
    finally:
        api.DisposeHandle(ctypes.c_void_p(handle.value))
    # Finder resolves this record when it draws the window, so ask the Alias
    # Manager now whether it leads to the picture: a record the volume cannot
    # answer for would leave the window blank.
    resolved = os.path.realpath(resolve_alias(record))
    if resolved != os.path.realpath(target_path):
        raise LayoutError("the alias resolves to %s, not %s" % (resolved, target_path))
    return record


def resolve_alias(record):
    api = core_services()
    handle = ctypes.c_void_p()
    if api.PtrToHand(record, ctypes.byref(handle), len(record)) != 0:
        raise LayoutError("PtrToHand failed")
    ref = ctypes.create_string_buffer(80)  # FSRef
    changed = ctypes.c_ubyte(0)
    if api.FSResolveAlias(None, handle, ref, ctypes.byref(changed)) != 0:
        raise LayoutError("FSResolveAlias failed")
    path = ctypes.create_string_buffer(1024)
    if api.FSRefMakePath(ref, path, 1024) != 0:
        raise LayoutError("FSRefMakePath failed")
    return path.value.decode("utf-8", "surrogateescape")


# --- .DS_Store ------------------------------------------------------------
#
# The file is a buddy-allocated address space: header, B-tree header, the
# allocator's root block and one leaf holding the records, as described in
# MyGo's cmd/mygo/dsstore.go. The records below are the same ones it writes,
# plus the picture.

def record_long(name, code, value):
    return (name, code, "long", struct.pack(">I", value))


def record_blob(name, code, value):
    return (name, code, "blob", value)


def record_type(name, code, value):
    return (name, code, "type", value)


def encode_record(record):
    name, code, kind, value = record
    units = name.encode("utf-16-be")
    out = struct.pack(">I", len(units) // 2) + units + code.encode() + kind.encode()
    if kind == "blob":
        out += struct.pack(">I", len(value))
    return out + value


def icon_location(name, x, y):
    return record_blob(name, "Iloc", struct.pack(">II", x, y) + b"\xff\xff\xff\xff\xff\xff\x00\x00")


def ds_store(records):
    records = sorted(records, key=lambda r: (r[0].lower(), r[1]))
    leaf = struct.pack(">II", 0, len(records)) + b"".join(encode_record(r) for r in records)
    if len(leaf) > 0x1000:
        raise LayoutError(".DS_Store records need %d bytes, more than one page" % len(leaf))

    dsdb, root, node = 0x20, 0x800, 0x2000
    file = bytearray(4 + 0x4000)
    file[3] = 1
    mem = memoryview(file)[4:]

    mem[0:4] = b"Bud1"
    struct.pack_into(">IIII", mem, 4, root, 0x800, root, 0)
    mem[16:32] = bytes([0, 0, 0x10, 0x0C, 0, 0, 0x87, 0, 0, 0x20, 0x0B, 0, 0, 0, 0, 0])

    struct.pack_into(">IIIII", mem, dsdb, 2, 0, len(records), 1, 0x1000)

    struct.pack_into(">I", mem, root, 3)
    for i, addr in enumerate([root | 11, dsdb | 5, node | 13]):
        struct.pack_into(">I", mem, root + 8 + 4 * i, addr)
    p = 8 + 256 * 4
    struct.pack_into(">I", mem, root + p, 1)
    mem[root + p + 4] = 4
    mem[root + p + 5 : root + p + 9] = b"DSDB"
    struct.pack_into(">I", mem, root + p + 9, 1)
    p += 13
    # Free lists by log2 size: carving the blocks above out of the initial
    # 2 GiB leaves one free block of each other size, at an offset equal to
    # its size.
    for width in range(32):
        if width < 6 or width in (11, 13, 31):
            p += 4
            continue
        struct.pack_into(">II", mem, root + p, 1, 1 << width)
        p += 8

    mem[node : node + len(leaf)] = leaf
    return bytes(file)


def window_layout(app_name, window, icon_size, app_pos, apps_pos, alias):
    window_bounds = "{{%d, %d}, {%d, %d}}" % (WINDOW_POSITION + window)
    bwsp = {
        "ShowStatusBar": False,
        "WindowBounds": window_bounds,
        "ContainerShowSidebar": False,
        "PreviewPaneVisibility": False,
        "SidebarWidth": 180,
        "ShowTabView": False,
        "ShowToolbar": False,
        "ShowPathbar": False,
        "ShowSidebar": False,
    }
    icvp = {
        "viewOptionsVersion": 1,
        "backgroundType": 2,
        "backgroundImageAlias": alias,
        "backgroundColorRed": 1.0,
        "backgroundColorGreen": 1.0,
        "backgroundColorBlue": 1.0,
        "gridOffsetX": 0.0,
        "gridOffsetY": 0.0,
        "gridSpacing": 100.0,
        "arrangeBy": "none",
        "showIconPreview": False,
        "showItemInfo": False,
        "labelOnBottom": True,
        "textSize": 16.0,
        "iconSize": float(icon_size),
        "scrollPositionX": 0.0,
        "scrollPositionY": 0.0,
    }
    return ds_store(
        [
            record_long(".", "vSrn", 1),
            record_blob(".", "bwsp", plistlib.dumps(bwsp, fmt=plistlib.FMT_BINARY)),
            record_blob(".", "icvp", plistlib.dumps(icvp, fmt=plistlib.FMT_BINARY)),
            record_type(".", "icvl", b"icnv"),  # icon view
            icon_location(app_name, *app_pos),
            icon_location("Applications", *apps_pos),
        ]
    )


# --- the disk image -------------------------------------------------------

def attach(dmg, mountpoint):
    out = run(
        "hdiutil", "attach", "-readwrite", "-noverify", "-noautoopen", "-nobrowse",
        "-mountpoint", mountpoint, dmg,
    )
    for line in out.splitlines():
        fields = line.split()
        if fields and fields[0].startswith("/dev/disk"):
            return fields[0]
    raise LayoutError("hdiutil attach %s printed no device" % dmg)


def detach(device):
    last = None
    for delay in (0, 2, 4, 8, 16):
        if delay:
            time.sleep(delay)
        try:
            run("hdiutil", "detach", device)
            return
        except LayoutError as err:
            if "Resource busy" not in str(err):
                raise
            last = err
    raise last


def style(dmg, background, background_2x, app_name, window, icon_size, app_pos, apps_pos):
    for tool in ("hdiutil", "tiffutil"):
        if not shutil.which(tool):
            raise LayoutError("%s not found; this needs macOS" % tool)

    work = tempfile.mkdtemp(prefix="dmg-layout-")
    try:
        rw = os.path.join(work, "rw.dmg")
        mountpoint = os.path.join(work, "mnt")
        os.mkdir(mountpoint)
        run("hdiutil", "convert", dmg, "-format", "UDRW", "-o", rw)
        device = attach(rw, mountpoint)
        try:
            if not os.path.isdir(os.path.join(mountpoint, app_name)):
                raise LayoutError("%s holds no %s" % (os.path.basename(dmg), app_name))
            folder = os.path.join(mountpoint, BACKGROUND_DIR)
            os.makedirs(folder, exist_ok=True)
            picture = os.path.join(folder, BACKGROUND_FILE)
            # A two-representation TIFF (1x and 2x) is how Finder shows a
            # sharp picture on Retina displays.
            run("tiffutil", "-cathidpicheck", background, background_2x, "-out", picture)
            alias = alias_record(mountpoint, picture)
            layout = window_layout(app_name, window, icon_size, app_pos, apps_pos, alias)
            with open(os.path.join(mountpoint, ".DS_Store"), "wb") as f:
                f.write(layout)
            # Created by macOS while the image is mounted, as mygo does.
            for name in (".fseventsd", ".Trashes", ".TemporaryItems"):
                shutil.rmtree(os.path.join(mountpoint, name), ignore_errors=True)
        finally:
            detach(device)
        # LZMA, as mygo uses; the volume name and its Finder icon survive.
        run("hdiutil", "convert", rw, "-format", "ULMO", "-ov", "-o", dmg)
    finally:
        shutil.rmtree(work, ignore_errors=True)


def main(argv):
    root = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
    assets = os.path.join(root, "assets", "dmg")

    parser = argparse.ArgumentParser(description="Set the Finder background of the app's .dmg.")
    parser.add_argument("dmg", help="disk image to restyle, in place")
    parser.add_argument("--background", default=os.path.join(assets, "background.png"))
    parser.add_argument("--background-2x", default=os.path.join(assets, "background@2x.png"))
    parser.add_argument("--app-name", default=DEFAULT_APP_NAME)
    parser.add_argument("--icon-size", type=int, default=DEFAULT_ICON_SIZE)
    parser.add_argument("--app", default="%d,%d" % DEFAULT_APP, help="app icon center, x,y")
    parser.add_argument("--applications", default="%d,%d" % DEFAULT_APPLICATIONS, help="link center, x,y")
    args = parser.parse_args(argv)

    def point(value, what):
        try:
            x, y = (int(v) for v in value.split(","))
        except ValueError:
            parser.error("%s must be x,y" % what)
        return (x, y)

    if sys.platform != "darwin":
        parser.error("this script needs macOS")
    for path in (args.dmg, args.background, args.background_2x):
        if not os.path.isfile(path):
            parser.error("%s: no such file" % path)

    try:
        width, height = image_size(args.background)
        two = image_size(args.background_2x)
        if two != (width * 2, height * 2):
            raise LayoutError(
                "%s is %dx%d, not twice %dx%d"
                % (args.background_2x, two[0], two[1], width, height)
            )
        style(
            args.dmg,
            args.background,
            args.background_2x,
            args.app_name,
            (width, height),
            args.icon_size,
            point(args.app, "--app"),
            point(args.applications, "--applications"),
        )
    except LayoutError as err:
        print("dmg-layout: %s" % err, file=sys.stderr)
        return 1
    print("dmg-layout: %s now opens on %s (%dx%d)" % (
        os.path.basename(args.dmg), os.path.basename(args.background), width, height))
    return 0


def image_size(path):
    """Returns the pixel size of a PNG, whose header every tool here writes."""
    with open(path, "rb") as f:
        header = f.read(24)
    if header[:8] != b"\x89PNG\r\n\x1a\n" or header[12:16] != b"IHDR":
        raise LayoutError("%s: not a PNG" % path)
    return struct.unpack(">II", header[16:24])


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
