package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

func run(cmdStr string, args ...string) error {
	cmd := exec.Command(cmdStr, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func main() {
	fmt.Println("=== 1. Formatting Code ===")
	if err := run("go", "fmt", "./..."); err != nil {
		fmt.Printf("go fmt failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n=== 2. Linting and Vetting Code ===")
	if err := run("go", "vet", "./..."); err != nil {
		fmt.Printf("go vet failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n=== 3. Running All Tests ===")
	if err := run("go", "test", "./..."); err != nil {
		fmt.Printf("go test failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n=== 4. Verifying Cross-Compilation (cmd/termtalk) ===")
	// Compile for current OS
	fmt.Printf("Compiling for local OS (%s/%s)...\n", runtime.GOOS, runtime.GOARCH)
	buildName := "build_test_local"
	if runtime.GOOS == "windows" {
		buildName += ".exe"
	}
	if err := run("go", "build", "-o", buildName, "./cmd/termtalk"); err != nil {
		fmt.Printf("local build failed: %v\n", err)
		_ = os.Remove(buildName)
		os.Exit(1)
	}
	_ = os.Remove(buildName)

	// Cross-compile for Windows (if not current OS)
	if runtime.GOOS != "windows" {
		fmt.Println("Cross-compiling for Windows (amd64)...")
		cmd := exec.Command("go", "build", "-o", "build_test_win.exe", "./cmd/termtalk")
		cmd.Env = append(os.Environ(), "GOOS=windows", "GOARCH=amd64")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Printf("Windows cross-compiling failed: %v\n", err)
			_ = os.Remove("build_test_win.exe")
			os.Exit(1)
		}
		_ = os.Remove("build_test_win.exe")
	}

	// Cross-compile for macOS (darwin amd64)
	if runtime.GOOS != "darwin" {
		fmt.Println("Cross-compiling for macOS (darwin/amd64)...")
		cmd := exec.Command("go", "build", "-o", "build_test_mac", "./cmd/termtalk")
		cmd.Env = append(os.Environ(), "GOOS=darwin", "GOARCH=amd64")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Printf("macOS cross-compiling failed: %v\n", err)
			_ = os.Remove("build_test_mac")
			os.Exit(1)
		}
		_ = os.Remove("build_test_mac")
	}

	fmt.Println("\n=== 5. Verifying Cross-Compilation (cmd/termtalk-relay) ===")
	relayBuildName := "relay_test_local"
	if runtime.GOOS == "windows" {
		relayBuildName += ".exe"
	}
	fmt.Printf("Compiling relay for local OS (%s/%s)...\n", runtime.GOOS, runtime.GOARCH)
	if err := run("go", "build", "-o", relayBuildName, "./cmd/termtalk-relay"); err != nil {
		fmt.Printf("relay local build failed: %v\n", err)
		_ = os.Remove(relayBuildName)
		os.Exit(1)
	}
	_ = os.Remove(relayBuildName)

	fmt.Println("\n=================================")
	fmt.Println("  Validation Successful (PASS)   ")
	fmt.Println("=================================")
}
