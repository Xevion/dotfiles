#!/usr/bin/env nu
use std

# This script is intended to backup the contents of my Viofo A229 dashcam to my computer, very quickly and efficiently.
# It is intended to be ran semi-rarely (every month or two), and is also a limited test of the Fish shell/scripting language.
# It is intended to be cross-platform, but targets Ubuntu 22.04 LTS (with WSL2 support).

# Configuration details
let host = "roman"  # The host to backup to. This is defined in the ~/.ssh/config file.
let host_path = "/mnt/user/media/backup/dashcam"  # The path on the remote host to backup to.

# Check for required commands
let required_commands = ["rsync", "sudo", "mount", "umount", "cmd.exe", "ssh"]
for cmd in $required_commands {
    if (which $cmd | length) == 0 {
        print $"Error: Required command ($cmd) not found."
        exit 1
    }
}

# Acquire the actual hostname of the defined host
let host_name: string = (ssh -G $host | lines | find -r "^hostname\\s+" | str trim | split column " " | get column2).0

# Check network connectivity to backup target
print "Checking network connectivity to backup server..."
try {
    ping -c 1 $host_name
} catch {
    print $"Error: Cannot reach backup server '($host_name)'"
    exit 1
}

# Check if backup destination exists and is writable
print "Checking backup destination..."
try {
    ssh $host "test -d $host_path && test -w $host_path"
} catch {
    print "Error: Backup destination is not accessible or writable"
    exit 1
}

# Check available space on backup destination
print "Checking available space on backup destination..."
let required_space = 10GB  # 10GB in bytes
try {
    let available_space = ssh $host $"df --output=avail /mnt/user" | lines | skip 1 | str trim | get 0 | append "KB" | str join " " | into filesize
    if $available_space < $required_space {
        print $"Error: Insufficient space on backup destination"
        print $"Required: ($required_space), Available: ($available_space)"
        exit 1
    }

    print $"Available space: ($available_space)"
} catch { |err|
    print $"Error: Could not check available space on backup destination: ($err.msg)"
    exit 1
}

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
    let target = $"($host):($host_path)/video"
    do -c { rsync -avh --progress --remove-source-files ($source) ($target) }
} catch {
    print $env.LAST_EXIT_CODE
}

print "Copying photo files..."
try {
    let source = $"/mnt/($letter)/DCIM/Photo"
    let target = $"($host):($host_path)/photo"
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