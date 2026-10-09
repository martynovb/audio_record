//go:build windows && embedded

package recorder

import _ "embed"

//go:embed assets/audio-capture-windows.exe
var embeddedWindowsHelper []byte
