# macOS Installation Guide

## Download and Install

1. Download the appropriate binary for your Mac:
   - Intel Macs: `remoteweather_vX.X.X_darwin_amd64.tar.gz`
   - Apple Silicon (M1/M2/M3): `remoteweather_vX.X.X_darwin_arm64.tar.gz`

2. Extract the archive:
   ```bash
   tar xzf remoteweather_vX.X.X_darwin_amd64.tar.gz
   ```

## Dealing with Gatekeeper

When you first run remoteweather, macOS may show a security warning. This is because the binary is ad-hoc signed but not notarized with Apple.

### Option 1: Quick Fix
Remove the quarantine flag:
```bash
xattr -d com.apple.quarantine remoteweather
```

### Option 2: System Preferences
1. Try to run `./remoteweather`
2. When blocked, go to System Settings > Privacy & Security
3. You'll see a message about remoteweather being blocked
4. Click "Open Anyway"

### Option 3: Right-click Method
1. Right-click (or Control-click) on the remoteweather binary
2. Select "Open" from the context menu
3. Click "Open" in the dialog that appears

## Running remoteweather

After dealing with Gatekeeper, you can run remoteweather normally:
```bash
./remoteweather -config /path/to/config.db
```

## Making it Permanent

To install system-wide:
```bash
sudo cp remoteweather /usr/local/bin/
sudo chmod +x /usr/local/bin/remoteweather
```