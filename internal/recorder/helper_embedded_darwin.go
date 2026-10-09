//go:build darwin && embedded

package recorder

import _ "embed"

//go:embed assets/audio-capture-macos
var embeddedMacOSHelper []byte
