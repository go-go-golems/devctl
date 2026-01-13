#!/usr/bin/env python3
"""Render ANSI screen captures into PNG screenshots.

Dependencies:
  pip install --user pillow pyte
"""

from __future__ import annotations

import argparse
from pathlib import Path

import pyte
from PIL import Image, ImageDraw, ImageFont

BASE_COLORS = {
    "black": "000000",
    "red": "cd0000",
    "green": "00cd00",
    "brown": "cdcd00",
    "blue": "0000ee",
    "magenta": "cd00cd",
    "cyan": "00cdcd",
    "white": "e5e5e5",
}
BRIGHT_COLORS = {
    "black": "7f7f7f",
    "red": "ff0000",
    "green": "00ff00",
    "brown": "ffff00",
    "blue": "5c5cff",
    "magenta": "ff00ff",
    "cyan": "00ffff",
    "white": "ffffff",
}


def resolve_color(value, bold, default_hex):
    if value is None:
        return "#" + default_hex
    if isinstance(value, int):
        return "#" + pyte.graphics.FG_BG_256[value]
    value = str(value)
    if value == "default":
        return "#" + default_hex
    if len(value) == 6 and all(c in "0123456789abcdef" for c in value):
        return "#" + value
    value = value.lower()
    if bold and value in BRIGHT_COLORS:
        return "#" + BRIGHT_COLORS[value]
    if value in BASE_COLORS:
        return "#" + BASE_COLORS[value]
    return "#" + default_hex


def render_ansi(
    ansi_path: Path,
    png_path: Path,
    cols: int,
    rows: int,
    font_path: str,
    font_size: int,
    bg_hex: str,
    fg_hex: str,
) -> None:
    data = ansi_path.read_text(encoding="utf-8", errors="ignore")
    screen = pyte.Screen(cols, rows)
    stream = pyte.Stream(screen)
    stream.feed(data)

    font = ImageFont.truetype(font_path, font_size)
    left, top, right, bottom = font.getbbox("M")
    cell_w = right - left
    cell_h = bottom - top
    # Offsets to account for font baseline/bearing in getbbox
    x_offset = left
    y_offset = top

    image = Image.new("RGB", (cols * cell_w, rows * cell_h), "#" + bg_hex)
    draw = ImageDraw.Draw(image)

    for y in range(rows):
        row = screen.buffer[y]
        for x in range(cols):
            ch = row[x]
            fg = resolve_color(ch.fg, ch.bold, fg_hex)
            bg = resolve_color(ch.bg, False, bg_hex)
            if ch.reverse:
                fg, bg = bg, fg
            if bg != "#" + bg_hex:
                draw.rectangle(
                    [x * cell_w, y * cell_h, (x + 1) * cell_w, (y + 1) * cell_h],
                    fill=bg,
                )
            char = ch.data if ch.data else " "
            if char != " ":
                # Subtract font bearing to align glyphs correctly in cells
                draw.text((x * cell_w - x_offset, y * cell_h - y_offset), char, font=font, fill=fg)

    image.save(png_path)


def main() -> None:
    parser = argparse.ArgumentParser(description="Convert ANSI screen dumps to PNGs.")
    parser.add_argument("--input-dir", default="docs/screenshots", help="Directory with .ansi files")
    parser.add_argument("--output-dir", default=None, help="Directory for .png output (defaults to input-dir)")
    parser.add_argument("--cols", type=int, default=120, help="Terminal columns")
    parser.add_argument("--rows", type=int, default=40, help="Terminal rows")
    parser.add_argument("--font", default="/usr/share/fonts/truetype/dejavu/DejaVuSansMono.ttf")
    parser.add_argument("--font-size", type=int, default=14)
    parser.add_argument("--bg", default="1c1c1c", help="Background hex (no #)")
    parser.add_argument("--fg", default="d0d0d0", help="Foreground hex (no #)")
    args = parser.parse_args()

    input_dir = Path(args.input_dir)
    output_dir = Path(args.output_dir) if args.output_dir else input_dir
    output_dir.mkdir(parents=True, exist_ok=True)

    for ansi_file in input_dir.glob("*.ansi"):
        png_file = output_dir / (ansi_file.stem + ".png")
        render_ansi(
            ansi_file,
            png_file,
            cols=args.cols,
            rows=args.rows,
            font_path=args.font,
            font_size=args.font_size,
            bg_hex=args.bg,
            fg_hex=args.fg,
        )
        print(f"Rendered {png_file}")


if __name__ == "__main__":
    main()
