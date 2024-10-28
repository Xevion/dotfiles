#!/usr/bin/env nu
use std

# This script is intended to backup the contents of my Viofo A229 dashcam to my computer, very quickly and efficiently.
# It is intended to be ran semi-rarely (every month or two), and is also a limited test of the Fish shell/scripting language.
# It is intended to be cross-platform, but targets Ubuntu 22.04 LTS (with WSL2 support).

# Acquire a list of potential mountable drive letters
let irrelevant = mount | grep drvfs | split column " " | get column1 | split column ":" | get column1
mut mountable = cmd.exe /C "wmic logicaldisk get name" e> (std null-device) | split row "\n" | skip 1 | split column ":" | get column1 | filter {|x| ($irrelevant | find $x | length) == 0}

print "Pick a USB device to mount for Viofo backup:"
let letter = $mountable | input list

let drive_letter = $"($letter):"

# Mount the drive
# TODO: Check if the drive is already mounted
sudo mkdir --parents /mnt/($letter)
sudo mount -t drvfs ($drive_letter) /mnt/($letter) -o uid=(id -u $env.USER),gid=(id -g $env.USER),metadata

# Test permissions & folder structure
let expected_folders = [
    "DCIM",
    "DCIM/Movie",
    "DCIM/Movie/RO",
    "DCIM/Photo"
]

print "Checking folder structure..."
for folder_suffix in $expected_folders {
    # Test folder existence
    let path = $"/mnt/($letter)/($folder_suffix)"
    let status = test -d ($path)  | complete
    if $status.exit_code != 0 {
        print $"Error: Expected folder ($path) does not exist."
        exit 1
    }

    # TODO: Test folder permissions (READ, EXECUTE required)
    # TODO: Test file permissions (all RO/Photo need READ/WRITE)
}

# Invoke rsync to copy the files
print "Copying video files..."
try {
    let source = $"/mnt/($letter)/DCIM/Movie/RO"
    let target = "roman:/mnt/user/media/dashcam/video"
    do -c { rsync -avh --progress --remove-source-files ($source) ($target) }
} catch {
    print $env.LAST_EXIT_CODE
}

print "Copying photo files..."
try {
    let source = $"/mnt/($letter)/DCIM/Photo"
    let target = "roman:/mnt/user/media/dashcam/photo"
    do -c { rsync -avh --progress --remove-source-files ($source) ($target) }
} catch {
    print $env.LAST_EXIT_CODE
}

print "Sync complete."

# Unmount the drive
print "Unmounting drive..."
sudo umount /mnt/($letter)
sudo rmdir /mnt/($letter)

# TODO: Check if duplicate mounts exist

print "All backed up."
    
# TODO: Statistical analysis of file duration
# On average, how far back do my recordings go? 2 months?
# While the oldest video file could give an idea of how back I currently go, it would be better to use p99 average bitrate.
# If you know the average bitrate, you can use the current space occupied by all files to estimate the total duration you could store on the drive.
# Then, if you can estimate the daily recording duration on average, you can estimate how many days back you can record on average.
# Additionally, since this script removes files, it might be better to use the 'full size' of the disk to estimate maximum duration capacity (minus 1GB).