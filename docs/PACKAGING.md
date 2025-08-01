# RemoteWeather Packaging Guide

This directory contains packaging files for distributing RemoteWeather on various Linux distributions.

## Arch Linux AUR Package

### Files
- **`PKGBUILD`** - Main package build script
- **`.SRCINFO`** - AUR metadata file (generated from PKGBUILD)
- **`remoteweather.install`** - Post-installation hooks
- **`remoteweather.service`** - systemd unit file

### Building the Package

1. **Update version information**:
   ```bash
   # Edit PKGBUILD and update:
   pkgver=1.0.0  # Current version
   pkgrel=1      # Package release number
   ```

2. **Generate checksums**:
   ```bash
   # Download source and calculate checksums
   makepkg --geninteg
   # Update sha256sums in PKGBUILD with the output
   ```

3. **Generate .SRCINFO**:
   ```bash
   makepkg --printsrcinfo > .SRCINFO
   ```

4. **Test the package**:
   ```bash
   # Build package locally
   makepkg -si
   
   # Test installation
   sudo systemctl status remoteweather
   ```

### AUR Submission

1. **Create AUR repository**:
   ```bash
   git clone ssh://aur@aur.archlinux.org/remoteweather.git
   cd remoteweather
   ```

2. **Add package files**:
   ```bash
   cp PKGBUILD .SRCINFO remoteweather.install remoteweather.service ./
   git add PKGBUILD .SRCINFO remoteweather.install remoteweather.service
   git commit -m "Initial import of remoteweather package"
   git push origin master
   ```

### Package Features

#### Installation Layout
```
/usr/bin/
├── remoteweather                    # Main application
├── remoteweather-config-convert     # YAML→SQLite converter
├── remoteweather-config-test        # Configuration validator
└── remoteweather-migrate            # Database migration tool

/var/lib/remoteweather/
└── config.yaml                      # Default configuration

/usr/lib/systemd/system/
└── remoteweather.service            # systemd unit

/usr/share/doc/remoteweather/
├── README.md                        # Main documentation
├── SQLITE_CONFIG_BACKEND.md         # SQLite backend guide
└── examples/                        # Example configurations

/usr/share/remoteweather/
└── migrations/                      # Database migration files
```

#### Security Features
- Dedicated `remoteweather` system user
- Runs with minimal privileges
- Protected directories and files
- systemd security hardening

#### Configuration Management
- Default YAML configuration in `/var/lib/remoteweather/config.yaml`
- SQLite conversion tools included
- Backup protection for configuration files
- Example configurations provided

## systemd Unit File

### Features
- **Security**: Runs as dedicated user with restricted permissions
- **Reliability**: Auto-restart on failure with rate limiting
- **Resource Management**: Memory and task limits
- **Logging**: Structured logging to systemd journal
- **Network**: Proper network dependency handling

### Usage
```bash
# Install and enable service
sudo systemctl enable remoteweather
sudo systemctl start remoteweather

# Check status
sudo systemctl status remoteweather

# View logs
sudo journalctl -u remoteweather -f

# Configuration reload (if supported)
sudo systemctl reload remoteweather
```

### Customization
The systemd unit defaults to SQLite configuration but can be modified:

```bash
# Edit service file
sudo systemctl edit remoteweather

# Add override:
[Service]
ExecStart=
ExecStart=/usr/bin/remoteweather -config /etc/remoteweather/config.yaml -config-backend yaml
```

## Other Distributions

### Debian/Ubuntu (.deb)
The systemd unit file can be used with minimal modifications for Debian packages:
- Install to `/lib/systemd/system/remoteweather.service`
- Create `remoteweather` user in postinst script
- Use `dh_systemd_enable` and `dh_systemd_start` helpers

### Red Hat/CentOS (.rpm)
Compatible with systemd-based RHEL distributions:
- Use `%systemd_post`, `%systemd_preun`, `%systemd_postun` macros
- Install to `/usr/lib/systemd/system/remoteweather.service`
- Create user with `useradd -r -s /sbin/nologin remoteweather`

### Docker
The application can also be containerized:
```dockerfile
FROM golang:alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o remoteweather .

FROM alpine:latest
RUN adduser -D -s /bin/false remoteweather
USER remoteweather
COPY --from=builder /app/remoteweather /usr/bin/
ENTRYPOINT ["/usr/bin/remoteweather"]
```

## Configuration Examples

### Basic Home Setup
```yaml
devices:
  - name: "home-weather"
    type: "davis"
    hostname: "192.168.1.100"
    port: "22222"

controllers:
  - type: "rest"
    rest:
      port: 8080
      weather-site:
        station-name: "Home Weather Station"
```

### Production with Database
```bash
# Convert to SQLite for production
remoteweather-config-convert \
  -yaml /var/lib/remoteweather/config.yaml \
  -sqlite /var/lib/remoteweather/config.db

# Update systemd to use SQLite (default)
sudo systemctl start remoteweather
```

## Maintenance

### Updates
1. Update `pkgver` in PKGBUILD
2. Update checksums: `makepkg --geninteg`
3. Regenerate `.SRCINFO`: `makepkg --printsrcinfo > .SRCINFO`
4. Test locally: `makepkg -si`
5. Commit and push to AUR

### Security
- Monitor for CVEs in Go dependencies
- Keep systemd security features updated
- Review file permissions regularly
- Update example configurations for security best practices

### Support
- Check systemd journal for errors: `journalctl -u remoteweather`
- Validate configuration: `remoteweather-config-test`
- Database issues: `remoteweather-migrate`
- Network connectivity: Check firewall and network settings 