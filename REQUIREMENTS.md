# PrintWireLabels - Requirements

## Overview

A Go desktop application using Gio UI that parses EPLAN terminal diagram Excel files and generates wire label output. The UI and project structure follows the pattern from [centroidx-manager](https://github.com/centroid-is/tfc-hmi/tree/main/tools/centroidx-manager).

## Input

- Excel file (`Terminal.xlsx`) containing terminal strip diagrams exported from EPLAN
- Each tab represents a terminal strip (e.g., `+U1-X13`, `+U2-X21`)
- Each tab has a consistent structure starting at row 7:

| Function text | Cable type | Target designation from | Connection | Terminal | Target designation to | Connection | Page / column |
|---|---|---|---|---|---|---|---|

### Data format

- **Target designation from/to** contains EPLAN references like `=GD1+U1-K4` or `+U1-X12`
- **Location designator** is the segment prefixed with `+` (e.g., `+U1`, `+U3`, `+U7.U7`)
- **Component designator** is the segment prefixed with `-` (e.g., `-K4`, `-X12`, `-X31`)
- **Connection** is the pin/terminal connection point (e.g., `A:10`, `1`, `B:2`)
- **Terminal** (column E) is the terminal number on the strip

## Workflow

1. **Select a tab** (terminal strip) from the Excel file
2. **Select a location designator** — the app extracts all unique `+` prefixed location designators found in that tab's "Target designation from" and "Target designation to" columns
3. **View output table** — filtered wire label data for the selected location

## Output

For each data row, generate a label pair showing both ends of the wire **within the selected cabinet/location**.

### Filtering logic

- The selected location (e.g., `+U1`) determines which cabinet you are wiring inside
- For each row, check "Target designation from" and "Target designation to" for the selected location prefix
- Extract the **component designator** (`-` prefixed) and **connection** from the side that matches the selected location
- The other side of the label is the terminal strip itself (strip designator + terminal number)
- Rows where neither side matches the selected location are excluded

### Example: Tab `+U1-X13`, Location `+U1`

Row: `Target from: =GD1+U1-K4 | Connection: A:10 | Terminal: 3 | Target to: +U3-X31 | Connection: 3`

The "from" side contains `+U1`, so the output is:

| Terminal Side | Component Side |
|---|---|
| `-X13:3` | `-K4:A:10` |

- **Terminal side**: strip designator + terminal number (e.g., `-X13:3`)
- **Component side**: component designator + connection from the side matching the selected location (e.g., `-K4:A:10`)
- Both terminal-to-component and terminal-to-terminal connections produce labels
- Only connections **within** the selected location/cabinet are shown — cross-cabinet references are excluded from output

## UI Design

Following the centroidx-manager pattern:

- **Solarized dark theme**
- **Left panel**: List of tabs (terminal strips) — clickable
- **Right panel (top)**: Dropdown or list of location designators for the selected tab
- **Right panel (bottom)**: Output table of wire labels

## Tech Stack

- **Language**: Go
- **UI**: Gio (gioui.org)
- **Excel parsing**: excelize or tealeg/xlsx
- **Project structure**: `internal/ui`, `internal/parser` (following centroidx-manager conventions)
