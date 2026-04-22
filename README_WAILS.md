# Wails Setup and Development Guide

This project is transitioning from Fyne to **Wails v2** for its GUI. Wails allows you to build desktop applications using Go for the backend and standard web technologies (HTML, CSS, JS) for the frontend.

## Prerequisites

Before you begin, ensure you have the following installed on your system:

1.  **Go:** [Download and Install Go](https://go.dev/doc/install) (v1.18 or later).
2.  **Node.js & NPM:** [Download and Install Node.js](https://nodejs.org/en/download/) (needed for the frontend).
3.  **Wails CLI:** Install the Wails command-line tool by running:
    ```bash
    go install github.com/wailsapp/wails/v2/cmd/wails@latest
    ```
    *Note: Ensure your Go bin directory (usually `~/go/bin` or `%USERPROFILE%\go\bin`) is in your system's PATH.*

## System-Specific Dependencies

Wails requires native development libraries to be present:

### Windows
-   **WebView2 Runtime:** Most modern Windows 10/11 systems already have this. If not, download it from [Microsoft](https://developer.microsoft.com/en-us/microsoft-edge/webview2/).

### macOS
-   **Xcode Command Line Tools:** Run `xcode-select --install`.

### Linux
-   **WebKit2Gtk:**
    -   Ubuntu/Debian: `sudo apt install libgtk-3-dev libwebkit2gtk-4.0-dev`
    -   Fedora: `sudo dnf install gtk3-devel webkit2gtk3-devel`

## Development

### 1. Initialize the Project
If starting from scratch or initializing a new structure:
```bash
wails init -n "nanogo" -t vanilla
```

### 2. Run in Development Mode
To start the application with "Hot Reload" (frontend changes update instantly):
```bash
wails dev
```
This will compile the Go code, install frontend dependencies, and open the application window.

### 3. Build for Production
To create a standalone, optimized executable:
```bash
wails build
```
The resulting binary will be in the `build/bin` directory.

## Project Structure
-   `main.go`: The entry point for the Wails application.
-   `app.go`: Contains the `App` struct where you define Go methods accessible to the frontend.
-   `frontend/`: Contains all your HTML, CSS, and Javascript code.
-   `wails.json`: Project configuration.

## Useful Links
-   [Wails Documentation](https://wails.io/docs/introduction)
-   [Wails CLI Commands](https://wails.io/docs/reference/cli)
