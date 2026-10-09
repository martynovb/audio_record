//go:build darwin

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
)

const helperEnvironmentVariable = "AUDIO_RECORD_MACOS_HELPER"

type darwinBackend struct{}

func New() Backend {
	return &darwinBackend{}
}

func (backend *darwinBackend) Preflight() error {
	return nil
}

func (backend *darwinBackend) Capture(
	ctx context.Context,
	config Config,
	stdout io.Writer,
	stderr io.Writer,
) error {
	helper, err := findOrBuildHelper(stdout, stderr)
	if err != nil {
		return err
	}

	args := []string{"--output", config.OutputPath}
	if config.ScreenID != "" {
		args = append(args, "--screen-id", config.ScreenID)
	}
	if config.MicrophoneID != "" {
		args = append(args, "--mic-id", config.MicrophoneID)
	}
	if !config.SystemAudio {
		args = append(args, "--no-system-audio")
	}
	if !config.Microphone {
		args = append(args, "--no-microphone")
	}

	command := exec.Command(helper, args...)
	command.Stdout = stdout
	command.Stderr = stderr
	if err := command.Start(); err != nil {
		return fmt.Errorf("start macOS capture helper: %w", err)
	}

	waitResult := make(chan error, 1)
	go func() {
		waitResult <- command.Wait()
	}()

	select {
	case err := <-waitResult:
		if err != nil {
			return fmt.Errorf("macOS capture helper: %w", err)
		}
		return nil
	case <-ctx.Done():
		if err := command.Process.Signal(os.Interrupt); err != nil && !errors.Is(err, os.ErrProcessDone) {
			return fmt.Errorf("stop macOS capture helper: %w", err)
		}
		if err := <-waitResult; err != nil {
			return fmt.Errorf("macOS capture helper: %w", err)
		}
		return nil
	}
}

func findOrBuildHelper(stdout, stderr io.Writer) (string, error) {
	if configured := os.Getenv(helperEnvironmentVariable); configured != "" {
		if isExecutable(configured) {
			return configured, nil
		}
		return "", fmt.Errorf("%s does not point to an executable: %s", helperEnvironmentVariable, configured)
	}
	if len(embeddedMacOSHelper) != 0 {
		return extractEmbeddedHelper()
	}
	if installed := findInstalledHelper(); installed != "" {
		return installed, nil
	}

	projectDirectory, err := findProjectDirectory()
	if err != nil {
		return "", err
	}

	packageDirectory := filepath.Join(projectDirectory, "platforms", "macos")
	helper := filepath.Join(packageDirectory, ".build", "release", "audio-capture-macos")
	if isExecutable(helper) {
		return helper, nil
	}

	if _, err := exec.LookPath("swift"); err != nil {
		return "", errors.New("Swift is required to build the macOS capture helper; install Xcode Command Line Tools")
	}

	fmt.Fprintln(stdout, "Building the macOS capture helper...")
	command := exec.Command("swift", "build", "--package-path", packageDirectory, "--configuration", "release")
	command.Stdout = stdout
	command.Stderr = stderr
	if err := command.Run(); err != nil {
		return "", fmt.Errorf("build macOS capture helper: %w", err)
	}
	if !isExecutable(helper) {
		return "", fmt.Errorf("build succeeded but helper was not found at %s", helper)
	}
	return helper, nil
}

func extractEmbeddedHelper() (string, error) {
	cacheDirectory, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("resolve user cache directory: %w", err)
	}
	return extractEmbeddedHelperTo(cacheDirectory)
}

func extractEmbeddedHelperTo(cacheDirectory string) (string, error) {
	digest := fmt.Sprintf("%x", sha256.Sum256(embeddedMacOSHelper))[:16]
	helperDirectory := filepath.Join(cacheDirectory, "audio-record", digest)
	if err := os.MkdirAll(helperDirectory, 0o700); err != nil {
		return "", fmt.Errorf("create helper cache directory: %w", err)
	}
	helper := filepath.Join(helperDirectory, "audio-capture-macos")
	if isExecutable(helper) {
		return helper, nil
	}

	temporary, err := os.CreateTemp(helperDirectory, "audio-capture-macos-")
	if err != nil {
		return "", fmt.Errorf("create temporary helper: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err := temporary.Write(embeddedMacOSHelper); err != nil {
		temporary.Close()
		return "", fmt.Errorf("write embedded helper: %w", err)
	}
	if err := temporary.Chmod(0o700); err != nil {
		temporary.Close()
		return "", fmt.Errorf("make embedded helper executable: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return "", fmt.Errorf("close embedded helper: %w", err)
	}
	if err := os.Rename(temporaryPath, helper); err != nil {
		if isExecutable(helper) {
			return helper, nil
		}
		return "", fmt.Errorf("install embedded helper: %w", err)
	}
	return helper, nil
}

func findInstalledHelper() string {
	executable, err := os.Executable()
	if err != nil {
		return ""
	}
	binDirectory := filepath.Dir(executable)
	candidates := []string{
		filepath.Join(binDirectory, "audio-capture-macos"),
		filepath.Join(binDirectory, "..", "libexec", "audio-record", "audio-capture-macos"),
	}
	for _, candidate := range candidates {
		if isExecutable(candidate) {
			return candidate
		}
	}
	return ""
}

func findProjectDirectory() (string, error) {
	candidates := make([]string, 0, 2)
	if workingDirectory, err := os.Getwd(); err == nil {
		candidates = append(candidates, workingDirectory)
	}
	if executable, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Dir(executable))
	}

	for _, candidate := range candidates {
		for {
			packageFile := filepath.Join(candidate, "platforms", "macos", "Package.swift")
			if _, err := os.Stat(packageFile); err == nil {
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
		"cannot find platforms/macos/Package.swift; run from the project directory or set %s",
		helperEnvironmentVariable,
	)
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir() && info.Mode()&0o111 != 0
}
