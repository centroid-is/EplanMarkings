# EplanMarkings

Go desktop application (Gio UI) that parses EPLAN terminal diagram Excel files and generates wire labels for cabinet wiring. Uses a Solarized dark theme.

![Screenshot](screenshot.png)

## Download

Pre-built binaries are available from the latest release:

| Platform | Download |
|----------|----------|
| Windows (amd64) | [EplanMarkings_windows_amd64.exe](https://github.com/centroid-is/EplanMarkings/releases/latest/download/EplanMarkings_windows_amd64.exe) |
| Linux (amd64) | [EplanMarkings_linux_amd64](https://github.com/centroid-is/EplanMarkings/releases/latest/download/EplanMarkings_linux_amd64) |
| macOS (arm64) | [EplanMarkings_darwin_arm64](https://github.com/centroid-is/EplanMarkings/releases/latest/download/EplanMarkings_darwin_arm64) |

## Features

- Excel terminal strip parsing
- Location-based filtering
- Wire label generation
- Weidmüller THM MultiMark network printing (cab JScript over TCP)

## Build from Source

```bash
go build -buildvcs=false .
```
