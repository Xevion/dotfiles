#!/usr/bin/env nu
use std
use std/log

# This script is intended to backup the contents of my Viofo A229 dashcam to my computer, very quickly and efficiently.
# It is intended to be ran semi-rarely (every month or two), and is also a limited test of the Fish shell/scripting language.
# It is intended to be cross-platform, but targets Ubuntu 22.04 LTS (with WSL2 support).

try { 
    # Configuration details
let host = "roman"  # The host to backup to. This is defined in the ~/.ssh/config file.
let host_path = "/mnt/user/media/backup/dashcam"  # The path on the remote host to backup to.

# Check for required commands
let required_commands = ["rsync", "sudo", "mount", "umount", "cmd.exe", "ssh"]
for cmd in $required_commands {
    if (which $cmd | length) == 0 {
        log error $"Error: Required command ($cmd) not found."
        exit 1
    }
}

# Acquire the actual hostname of the defined host
let host_name: string = (ssh -G $host | lines | find -r "^hostname\\s+" | str trim | split column " " | get column2).0

# Check network connectivity to backup target
log debug "Checking network connectivity to backup server..."
let ping_check = ping -c 1 $host_name | complete
if $ping_check.exit_code != 0 {
    log error $"Error: Could not verify network connectivity to '($host_name)'"
    log error $ping_check.stderr
    exit 1
}

# Check if backup destination exists and is writable
log debug "Checking backup destination..."
try {
    ssh $host "test -d $host_path && test -w $host_path"
} catch {
    log error "Error: Backup destination is not accessible or writable"
    exit 1
}

# Check available space on backup destination
log debug "Checking available space on backup destination..."
let required_space = 10GB  # 10GB in bytes
try {
    let available_space = ssh $host $"df --output=avail /mnt/user" | lines | skip 1 | str trim | get 0 | append "KB" | str join " " | into filesize
    if $available_space < $required_space {
        log error $"Error: Insufficient space on backup destination"
        log error $"Required: ($required_space), Available: ($available_space)"
        exit 1
    }

    log debug $"Available space: ($available_space)"
} catch { |err|
    log error $"Error: Could not check available space on backup destination: ($err.msg)"
    exit 1
}

# Acquire a list of potential mountable drive letters
let mountable = (cd /mnt/c; cmd.exe /C "wmic logicaldisk where DriveType=2 get DeviceID,Name /format:csv" | from csv)
log info "Pick a USB device to mount for Viofo backup:"

# Check that at least one drive is available
if ($mountable | length) == 0 {
    log error "No USB drives found"
    exit 1
}

# Have the user choose a drive
let selected_drive = $mountable | input list -d "DeviceID" "Select a drive to mount for backup"
let letter = $selected_drive.DeviceId | str replace --regex ":$" "" | str downcase
let win_drive_path = $"($letter | str upcase):"

# Check if already mounted
log info "Checking if drive is already mounted..."
let findmnt_check = findmnt -J --mountpoint /mnt/($letter) | complete
if $findmnt_check.exit_code == 0 {
    log debug "Drive is already mounted"
   
    # Check was successful, meaning something must be mounted there
    let mounts = $findmnt_check.stdout | from json
    log debug "Mount JSON acquired"
    
    # If multiple mounts are found, print them, then exit
    if ($mounts.filesystems | length) > 1 {
        log error ("Multiple mounts found, cannot continue \n" + ($mounts | to json --indent 2))
        exit 1
    }
    log debug $"($mounts.filesystems | length) mounts found"

    let current_mount = $mounts.filesystems.0
    log debug $"Current mount: ($current_mount)"

    # Check that the mount is probably already correct
    if $current_mount.source != $win_drive_path {
        log error "Error: Drive is mounted at incorrect path"
        log error $"Mount Source expected ($win_drive_path), but found: ($current_mount.source)"
        exit 1
    } else if ($current_mount.options | str contains "rw") == false {
        log error "Error: Drive is not mounted read-write"
        log error $"Mount options: ($current_mount.options)"
        exit 1
    }

    log info $"Mount Details: ($current_mount.options)"
    while true {
        let continue = input "Continue anyways? (y/n)"
        if $continue == "y" {
            break
        } else if $continue == "n" {
            log error "User declined to continue"
            exit 1
        }
    }
}

    try { 
        # Mount the drive
    log info "Mounting drive (requires sudo)..."

    try {
        log debug "Preparing mount point folder..."
        sudo mkdir --parents /mnt/($letter)
    } catch { |err|
        log error $"Error: Could not prepare mount point folder: ($err.msg)"
        exit 1
    }

    try {
        log debug "Mounting drive..."
        sudo mount -t drvfs ($win_drive_path) /mnt/($letter) -o uid=(id -u $env.USER),gid=(id -g $env.USER),metadata
    } catch { |err|
        log error $"Error: Could not mount drive: ($err.msg)"
        exit 1
    }

    log debug "Drive mounted"

    # Verify mount was created
    log debug "Verifying mount was successful..."
    let mount_check = findmnt -J --mountpoint /mnt/($letter) | complete
    if $mount_check.exit_code != 0 {
        log error $mount_check.stderr
        error "Failed to mount drive"
    }

    # Test permissions & folder structure
    let expected_folders = [
        "DCIM",
        "DCIM/Movie",
        "DCIM/Movie/RO",
        "DCIM/Photo"
    ]

    # We don't need to check permissions, the mount won't support them (generally)
    log debug "Checking folder structure..."
    for folder_suffix in $expected_folders {
        # Test folder existence
        let path = $"/mnt/($letter)/($folder_suffix)"
        let status = test -d ($path)  | complete
        if $status.exit_code != 0 {
            error $"Expected folder "($path)" does not exist."
            
        } else {
            log debug $"Folder "($path)" exists"
        }
    }

    # Get total size of files to copy
    log debug "Calculating total size of files..."

    let video_source = $"/mnt/($letter)/DCIM/Movie/RO"
    let video_target = $"($host):($host_path)/video"
    let video_size = du $video_source | get 0 | get apparent | into filesize

    log info $"Video size: ($video_size)"

    let photo_source = $"/mnt/($letter)/DCIM/Photo"
    let photo_target = $"($host):($host_path)/photo"
    let photo_size = du $photo_source | get 0 | get apparent | into filesize

    log info $"Photo size: ($photo_size)"
    log info $"Total size: ($video_size + $photo_size | into filesize)"

    # Invoke rsync to copy the files
    log info "Copying video files..."
    try {
        do -c { rsync -avh --progress --remove-source-files ($video_source) ($video_target) }
    } catch {
        error $"Could not copy video files: ($env.LAST_EXIT_CODE)"
    }

    log info "Copying photo files..."
    try {
        do -c { rsync -avh --progress --remove-source-files ($photo_source) ($photo_target) }
    } catch {
        error $"Could not copy photo files: ($env.LAST_EXIT_CODE)"
    }

    log info "Sync complete."

    # Unmount the drive
    log info "Unmounting drive..."
    sudo umount /mnt/($letter)
    sudo rmdir /mnt/($letter)
    
    # TODO: Check if duplicate mounts exist
    log info "All backed up."
} catch { |err|
    log error $"Error: Could not unmount drive: ($err.msg)"
    exit 1
}


    
# TODO: Statistical analysis of file duration
# On average, how far back do my recordings go? 2 months?
# While the oldest video file could give an idea of how back I currently go, it would be better to use p99 average bitrate.
# If you know the average bitrate, you can use the current space occupied by all files to estimate the total duration you could store on the drive.
# Then, if you can estimate the daily recording duration on average, you can estimate how many days back you can record on average.
# Additionally, since this script removes files, it might be better to use the 'full size' of the disk to estimate maximum duration capacity (minus 1GB).
 } catch {|err|
    log error $"Uncaught error: ($err.msg)"
    exit 1
 }