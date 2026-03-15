#!/usr/bin/env python3
"""
Enhanced CLI utility to rename media files in a folder to a consistent pattern with date/time.

Now supports separate patterns for images vs. videos via optional arguments:
    --image-pattern <STR>
    --video-pattern <STR>

We can also clean (sanitize) prefix patterns by removing extra special characters/spaces.
You can enable this with:
    --clean-prefix

Additionally, we handle incorrectly named files by:
1. Strictly checking whether a file is "properly named" or not.
2. Adding a --force-reorganize flag so that any file that is NOT strictly valid gets renamed.

USAGE:
    python rename_script.py /path/to/folder

OPTIONS:
    --pattern <STR>         Prefix pattern (default 'media')
    --image-pattern <STR>   Prefix pattern for image files (optional)
    --video-pattern <STR>   Prefix pattern for video files (optional)
    --clean-prefix          If set, remove excessive special chars/spaces from prefix(es)
    --prompt-rename         Prompt the user if file is recognized as 'already organized'
    --dry-run               Show what would happen, but do not rename
    --use-file-ctime        Use file creation time for the date/time portion
    --force-reorganize      Rename files unless they strictly match the final naming pattern

EXAMPLE:
    python rename_script.py /path/to/folder \
        --image-pattern=pic \
        --video-pattern=clip \
        --force-reorganize \
        --clean-prefix
"""

import os
import re
import argparse
from datetime import datetime

# Known image and video extensions (simple approach)
IMAGE_EXTENSIONS = {'.png', '.jpg', '.jpeg', '.gif', '.bmp', '.tiff', '.webp', '.heic'}
VIDEO_EXTENSIONS = {'.mp4', '.mov', '.avi', '.mkv', '.wmv', '.flv', '.webm', '.m4v'}

# Regex to check strict final naming pattern: prefix__YYYYMMDD_HHMMSS_counter.extension
STRICT_PATTERN = re.compile(r'^[A-Za-z0-9_-]+__\d{8}_\d{6}_\d+\.[^.]+$')


def is_strictly_valid_name(fname: str) -> bool:
    """
    Return True if the entire filename (excluding any directory path) matches
    our final naming scheme. For example:
        myMedia__20250330_173000_12.mp4
    """
    return bool(STRICT_PATTERN.match(fname))


def clean_prefix(prefix: str) -> str:
    """
    Remove or replace excessive special characters/spaces from the prefix.
    1. Remove anything that's not alphanumeric, underscore, or dash.
    2. Convert multiple underscores to a single underscore.
    3. Trim leading/trailing underscores.
    """
    # remove disallowed chars
    cleaned = re.sub(r'[^A-Za-z0-9_-]+', '', prefix)
    # collapse multiple underscores into one
    cleaned = re.sub(r'_{2,}', '_', cleaned)
    # strip leading/trailing underscores
    cleaned = cleaned.strip('_')
    return cleaned


def main():
    parser = argparse.ArgumentParser(
        description=(
            "Enhanced rename script with separate patterns for images, videos, "
            "and extra checks for incorrectly named files."
        )
    )
    parser.add_argument("folder", help="Folder containing media files", type=str)
    parser.add_argument("--pattern", help="Default pattern prefix (default 'media')", default="media", type=str)
    parser.add_argument("--image-pattern", help="Pattern prefix for image files", default=None, type=str)
    parser.add_argument("--video-pattern", help="Pattern prefix for video files", default=None, type=str)
    parser.add_argument("--clean-prefix", help="Remove extra special characters/spaces from prefix(es)", action="store_true")
    parser.add_argument("--prompt-rename", help="Prompt user if file is recognized as 'already organized'", action="store_true")
    parser.add_argument("--dry-run", help="Only show what would happen without actually renaming", action="store_true")
    parser.add_argument("--use-file-ctime", help="Use file creation time for the date/time portion", action="store_true")
    parser.add_argument(
        "--force-reorganize", 
        help="Rename any file that does not strictly match the final pattern.",
        action="store_true"
    )

    args = parser.parse_args()
    folder = args.folder
    default_pattern = args.pattern
    image_pattern = args.image_pattern
    video_pattern = args.video_pattern
    do_clean_prefix = args.clean_prefix
    prompt_rename = args.prompt_rename
    dry_run = args.dry_run
    use_file_ctime = args.use_file_ctime
    force_reorg = args.force_reorganize

    # Optionally clean user-supplied patterns
    if do_clean_prefix:
        default_pattern = clean_prefix(default_pattern)
        if image_pattern is not None:
            image_pattern = clean_prefix(image_pattern)
        if video_pattern is not None:
            video_pattern = clean_prefix(video_pattern)

    if not os.path.isdir(folder):
        print(f"Error: '{folder}' is not a valid directory.")
        return

    all_files = sorted(os.listdir(folder))

    counter = 0

    for fname in all_files:
        old_path = os.path.join(folder, fname)
        if not os.path.isfile(old_path):
            continue  # skip directories, etc.

        # skip hidden/system files
        if fname.startswith('.'):
            continue

        # Determine extension (lowercase for checking, but keep original in final rename)
        _, ext = os.path.splitext(fname)
        ext_lower = ext.lower()

        # Decide which prefix to use based on extension
        if ext_lower in IMAGE_EXTENSIONS and image_pattern:
            prefix_pattern = image_pattern
        elif ext_lower in VIDEO_EXTENSIONS and video_pattern:
            prefix_pattern = video_pattern
        else:
            # If it's not recognized or no specialized pattern is set, fallback to default
            prefix_pattern = default_pattern

        # Check if filename is already strictly valid.
        if is_strictly_valid_name(fname):
            # If strictly valid, only rename if user wants to forcibly reorg everything
            if force_reorg:
                # We'll rename it anyway, generating a fresh date/time and new counter.
                pass
            else:
                # It's strictly valid and we're not forcing reorg, so skip.
                continue
        else:
            # The file is not strictly valid.
            partial_date_match = re.search(r'__\d{8}_\d{6}', fname) is not None
            # If partial date/time substring is found, we can either prompt or rename.
            if partial_date_match and prompt_rename and not force_reorg:
                answer = input(f"File '{fname}' looks partially organized but isn't strictly valid. Rename? [y/N]: ")
                if answer.lower() != 'y':
                    continue
            elif partial_date_match and not prompt_rename and not force_reorg:
                # old script's default was to skip if it found a date/time snippet.
                # But we want to reorganize anyway if it's not strictly valid.
                # Let's decide here: if not forced or prompted, we skip.
                continue
            else:
                # no partial match or we are free to rename (force reorg, etc.). do nothing special.
                pass

        # At this point, we want to rename the file.

        # Generate date/time.
        if use_file_ctime:
            ctime_ts = os.path.getctime(old_path)
            dt_str = datetime.fromtimestamp(ctime_ts).strftime("%Y%m%d_%H%M%S")
        else:
            dt_str = datetime.now().strftime("%Y%m%d_%H%M%S")

        # Build new filename
        new_fname = f"{prefix_pattern}__{dt_str}_{counter}{ext}"

        # If desired, we can do a second pass to ensure the final name doesn't have repeated underscores:
        # (But be careful not to remove the double underscore after prefix, if you want to keep that design.)
        # For example:
        # final_fname = re.sub(r'_{2,}', '_', new_fname)  # merges consecutive underscores
        # if you want to preserve the prefix + __ structure exactly, you might do something more nuanced.
        # We'll keep it simple for now.

        new_path = os.path.join(folder, new_fname)

        if dry_run:
            print(f"[DRY RUN] '{fname}' -> '{new_fname}'")
        else:
            os.rename(old_path, new_path)
            print(f"Renamed: '{fname}' -> '{new_fname}'")

        counter += 1

if __name__ == "__main__":
    main()

