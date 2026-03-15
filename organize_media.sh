#!/bin/bash

# Default settings
GRANULARITY="month"  # Options: year, month, day
RENAME_WITH_DATE=false
PRESERVE_PREFIX=true

# Function to display usage
usage() {
    echo "Usage: $0 <directory> [options]"
    echo "Options:"
    echo "  -g, --granularity <year|month|day>  Set organization granularity (default: month)"
    echo "  -r, --rename                        Rename files with timestamp prefix"
    echo "  -n, --no-preserve-prefix            Don't preserve original filename prefix"
    echo "  -h, --help                          Display this help message"
    exit 1
}

# Parse command line arguments
SOURCE_DIR=""
while [[ $# -gt 0 ]]; do
    case "$1" in
        -g|--granularity)
            GRANULARITY="$2"
            if [[ ! "$GRANULARITY" =~ ^(year|month|day)$ ]]; then
                echo "Error: Granularity must be 'year', 'month', or 'day'"
                usage
            fi
            shift 2
            ;;
        -r|--rename)
            RENAME_WITH_DATE=true
            shift
            ;;
        -n|--no-preserve-prefix)
            PRESERVE_PREFIX=false
            shift
            ;;
        -h|--help)
            usage
            ;;
        *)
            if [[ -z "$SOURCE_DIR" ]]; then
                SOURCE_DIR="$1"
                shift
            else
                echo "Error: Unknown option $1"
                usage
            fi
            ;;
    esac
done

# Check if directory is provided
if [ -z "$SOURCE_DIR" ]; then
    echo "Error: No directory specified"
    usage
fi

# Check if directory exists
if [ ! -d "$SOURCE_DIR" ]; then
    echo "Error: Directory '$SOURCE_DIR' does not exist"
    exit 1
fi

# Check if exiftool is installed
if ! command -v exiftool &> /dev/null; then
    echo "Error: exiftool is not installed. Please install it first."
    exit 1
fi

# Create a temporary list of files to process
find "$SOURCE_DIR" -maxdepth 1 -type f -not -path "*/\.*" \
    \( -iname "*.jpg" -o -iname "*.jpeg" -o -iname "*.png" -o -iname "*.gif" -o -iname "*.heic" -o \
       -iname "*.mp4" -o -iname "*.mov" -o -iname "*.avi" -o -iname "*.m4v" \) > /tmp/files_to_organize.txt

# Function to extract the prefix from a filename
extract_prefix() {
    local filename="$1"

    # Try to extract prefix before " - " or " ("
    if [[ "$filename" == *" - "* ]]; then
        echo "${filename%% - *}"
    elif [[ "$filename" == *" ("* ]]; then
        echo "${filename%% (*}"
    else
        # If no pattern found, use the filename without extension
        echo "${filename%.*}"
    fi
}

# Process each file from the list
while read -r file; do
    # Get filename
    filename=$(basename "$file")
    extension="${filename##*.}"

    # Try to extract date from filename if it contains a date pattern (like T164608.950)
    if [[ "$filename" =~ ([0-9]{4})-([0-9]{2})-([0-9]{2})T([0-9]{6})\.([0-9]{3}) ]]; then
        year="${BASH_REMATCH[1]}"
        month="${BASH_REMATCH[2]}"
        day="${BASH_REMATCH[3]}"
        time="${BASH_REMATCH[4]}"
    else
        # Get creation date using exiftool
        date_info=$(exiftool -s3 -DateTimeOriginal -CreateDate -FileModifyDate "$file" 2>/dev/null | head -n 1)

        # If no date found, use file modification time
        if [ -z "$date_info" ]; then
            # Use different stat format based on OS
            if [[ "$OSTYPE" == "darwin"* ]]; then
                # macOS
                date_info=$(stat -f "%Sm" -t "%Y:%m:%d %H:%M:%S" "$file")
            else
                # Linux
                date_info=$(stat -c "%y" "$file" | cut -d. -f1)
            fi
        fi

        # Extract year, month, day and time
        if [[ "$date_info" =~ ([0-9]{4})[:-]([0-9]{2})[:-]([0-9]{2})\ ([0-9]{2})[:-]([0-9]{2})[:-]([0-9]{2}) ]]; then
            year="${BASH_REMATCH[1]}"
            month="${BASH_REMATCH[2]}"
            day="${BASH_REMATCH[3]}"
            hour="${BASH_REMATCH[4]}"
            min="${BASH_REMATCH[5]}"
            sec="${BASH_REMATCH[6]}"
            time="${hour}${min}${sec}"
        else
            echo "Could not parse date format for: $file"
            year="0000"
            month="00"
            day="00"
            time="000000"
        fi
    fi

    # Skip if we couldn't determine date
    if [ -z "$year" ] || [ -z "$month" ]; then
        echo "Could not determine date for: $file"
        continue
    fi

    # Determine target directory based on granularity
    target_dir="$SOURCE_DIR"
    case "$GRANULARITY" in
        year)
            target_dir="$SOURCE_DIR/$year"
            ;;
        month)
            target_dir="$SOURCE_DIR/$year/$month"
            ;;
        day)
            target_dir="$SOURCE_DIR/$year/$month/$day"
            ;;
    esac

    # Create target directory if it doesn't exist
    mkdir -p "$target_dir"

    # Determine new filename
    new_filename=""

    if [ "$PRESERVE_PREFIX" = true ]; then
        # Extract the prefix from the original filename
        prefix=$(extract_prefix "$filename")
        # Clean the prefix (replace spaces and special chars with underscores)
        clean_prefix=$(echo "$prefix" | tr ' ' '_' | tr -c '[:alnum:]_.' '_')

        if [ "$RENAME_WITH_DATE" = true ]; then
            new_filename="${clean_prefix}_${year}${month}${day}_${time}.${extension}"
        else
            new_filename="${clean_prefix}.${extension}"
        fi
    else
        if [ "$RENAME_WITH_DATE" = true ]; then
            new_filename="${year}${month}${day}_${time}.${extension}"
        else
            new_filename="$filename"
        fi
    fi

    # Check if the file already exists in the target directory
    if [ -f "$target_dir/$new_filename" ]; then
        counter=1
        while [ -f "$target_dir/${new_filename%.*}_${counter}.${extension}" ]; do
            counter=$((counter+1))
        done
        new_filename="${new_filename%.*}_${counter}.${extension}"
    fi

    # Move file to the target directory
    echo "Moving $filename to $target_dir/$new_filename"
    mv "$file" "$target_dir/$new_filename"
done < /tmp/files_to_organize.txt

rm /tmp/files_to_organize.txt
echo "Organization complete!"

