//go:build windows

package recorder

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

const windowsHelperEnvironmentVariable = "AUDIO_RECORD_WINDOWS_HELPER"

type windowsBackend struct{}

func New() Backend {
	return &windowsBackend{}
}

func (backend *windowsBackend) Preflight() error {
	return nil
}

func (backend *windowsBackend) Capture(
	ctx context.Context,
	config Config,
	stdout io.Writer,
	stderr io.Writer,
) error {
	helper, err := findOrBuildWindowsHelper(stdout, stderr)
	if err != nil {
		return err
	}

	arguments := []string{"--output", config.OutputPath}
	if config.MicrophoneID != "" {
		arguments = append(arguments, "--mic-id", config.MicrophoneID)
	}
	if !config.SystemAudio {
		arguments = append(arguments, "--no-system-audio")
	}
	if !config.Microphone {
		arguments = append(arguments, "--no-microphone")
	}

	command := exec.Command(helper, arguments...)
	command.Stdout = stdout
	command.Stderr = stderr
	stopInput, err := command.StdinPipe()
	if err != nil {
		return fmt.Errorf("create Windows helper stop pipe: %w", err)
	}
	if err := command.Start(); err != nil {
		return fmt.Errorf("start Windows capture helper: %w", err)
	}

	waitResult := make(chan error, 1)
	go func() {
		waitResult <- command.Wait()
	}()

	select {
	case err := <-waitResult:
		if err != nil {
			return fmt.Errorf("Windows capture helper: %w", err)
		}
		return nil
	case <-ctx.Done():
		_, _ = io.WriteString(stopInput, "stop\n")
		_ = stopInput.Close()

		select {
		case err := <-waitResult:
			if err != nil {
				return fmt.Errorf("Windows capture helper: %w", err)
			}
			return nil
		case <-time.After(30 * time.Second):
			_ = command.Process.Kill()
			<-waitResult
			return errors.New("Windows capture helper did not stop within 30 seconds")
		}
	}
}

func findOrBuildWindowsHelper(stdout, stderr io.Writer) (string, error) {
	if configured := os.Getenv(windowsHelperEnvironmentVariable); configured != "" {
		if isWindowsHelper(configured) {
			return configured, nil
		}
		return "", fmt.Errorf(
			"%s does not point to a file: %s",
			windowsHelperEnvironmentVariable,
			configured,
		)
	}
	if len(embeddedWindowsHelper) != 0 {
		return extractEmbeddedWindowsHelper()
	}
	if installed := findInstalledWindowsHelper(); installed != "" {
		return installed, nil
	}

	projectDirectory, err := findWindowsProjectDirectory()
	if err != nil {
		return "", err
	}
	if _, err := exec.LookPath("dotnet"); err != nil {
		return "", errors.New(".NET SDK is required to build the Windows capture helper")
	}

	runtimeIdentifier, err := windowsRuntimeIdentifier()
	if err != nil {
		return "", err
	}
	projectFile := filepath.Join(projectDirectory, "platforms", "windows", "AudioCaptureWindows.csproj")
	publishDirectory := filepath.Join(projectDirectory, "platforms", "windows", ".build", runtimeIdentifier)
	helper := filepath.Join(publishDirectory, "audio-capture-windows.exe")

	fmt.Fprintln(stdout, "Building the Windows capture helper...")
	command := exec.Command(
		"dotnet", "publish", projectFile,
		"--configuration", "Release",
		"--runtime", runtimeIdentifier,
		"--self-contained", "true",
		"--output", publishDirectory,
	)
	command.Stdout = stdout
	command.Stderr = stderr
	if err := command.Run(); err != nil {
		return "", fmt.Errorf("build Windows capture helper: %w", err)
	}
	if !isWindowsHelper(helper) {
		return "", fmt.Errorf("build succeeded but helper was not found at %s", helper)
	}
	return helper, nil
}

func extractEmbeddedWindowsHelper() (string, error) {
	cacheDirectory, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("resolve user cache directory: %w", err)
	}
	return extractEmbeddedWindowsHelperTo(cacheDirectory)
}

func extractEmbeddedWindowsHelperTo(cacheDirectory string) (string, error) {
	digest := fmt.Sprintf("%x", sha256.Sum256(embeddedWindowsHelper))[:16]
	helperDirectory := filepath.Join(cacheDirectory, "audio-record", digest)
	if err := os.MkdirAll(helperDirectory, 0o700); err != nil {
		return "", fmt.Errorf("create helper cache directory: %w", err)
	}
	helper := filepath.Join(helperDirectory, "audio-capture-windows.exe")
	if isWindowsHelper(helper) {
		return helper, nil
	}

	temporary, err := os.CreateTemp(helperDirectory, "audio-capture-windows-*.exe")
	if err != nil {
		return "", fmt.Errorf("create temporary Windows helper: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err := temporary.Write(embeddedWindowsHelper); err != nil {
		temporary.Close()
		return "", fmt.Errorf("write embedded Windows helper: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return "", fmt.Errorf("close embedded Windows helper: %w", err)
	}
	if err := os.Rename(temporaryPath, helper); err != nil {
		if isWindowsHelper(helper) {
			return helper, nil
		}
		return "", fmt.Errorf("install embedded Windows helper: %w", err)
	}
	return helper, nil
}

func findInstalledWindowsHelper() string {
	executable, err := os.Executable()
	if err != nil {
		return ""
	}
	binDirectory := filepath.Dir(executable)
	for _, candidate := range []string{
		filepath.Join(binDirectory, "audio-capture-windows.exe"),
		filepath.Join(binDirectory, "..", "libexec", "audio-record", "audio-capture-windows.exe"),
	} {
		if isWindowsHelper(candidate) {
			return candidate
		}
	}
	return ""
}

func findWindowsProjectDirectory() (string, error) {
	candidates := make([]string, 0, 2)
	if workingDirectory, err := os.Getwd(); err == nil {
		candidates = append(candidates, workingDirectory)
	}
	if executable, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Dir(executable))
	}

	for _, candidate := range candidates {
		for {
			projectFile := filepath.Join(candidate, "platforms", "windows", "AudioCaptureWindows.csproj")
			if _, err := os.Stat(projectFile); err == nil {
				return candidate, nil
			}
			parent := filepath.Dir(candidate)
			if parent == candidate {
				break
			}
			candidate = parent
		}
	}
	return "", fmt.Errorf(
		"cannot find platforms/windows/AudioCaptureWindows.csproj; set %s",
		windowsHelperEnvironmentVariable,
	)
}

func windowsRuntimeIdentifier() (string, error) {
	switch runtime.GOARCH {
	case "amd64":
		return "win-x64", nil
	case "arm64":
		return "win-arm64", nil
	default:
		return "", fmt.Errorf("unsupported Windows architecture: %s", runtime.GOARCH)
	}
}

func isWindowsHelper(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir() && info.Size() > 0
}
