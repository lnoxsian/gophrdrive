package config

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/lnoxsian/gophrdrv/internal/version"
)

type Config struct {
	Root         string
	Port         int
	Host         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	MaxUpload    int64 // in bytes
}

func ParseConfig() (*Config, error) {
	rootFlag := flag.String("root", "", "Filesystem root directory")
	portFlag := flag.Int("port", 8080, "Port to listen on")
	hostFlag := flag.String("host", "0.0.0.0", "Host to bind to")
	readTimeoutFlag := flag.Duration("read-timeout", 30*time.Second, "Read timeout duration")
	writeTimeoutFlag := flag.Duration("write-timeout", 30*time.Second, "Write timeout duration")
	maxUploadFlag := flag.String("max-upload", "100MB", "Maximum upload size (e.g. 100MB, 1GB)")
	versionFlag := flag.Bool("version", false, "Print version and exit")
	vFlag := flag.Bool("v", false, "Print version and exit")

	flag.Parse()

	if *versionFlag || *vFlag {
		fmt.Printf("GOPHRDRV version %s\n", version.Version)
		os.Exit(0)
	}

	// Handle root directory
	root := *rootFlag
	if root == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get current working directory: %w", err)
		}
		root = cwd
	}

	// Resolve absolute path for security checks
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve absolute path of root: %w", err)
	}

	// Clean the path
	absRoot = filepath.Clean(absRoot)

	// Verify root exists and is a directory
	info, err := os.Stat(absRoot)
	if err != nil {
		return nil, fmt.Errorf("root directory error: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("root path is not a directory: %s", absRoot)
	}

	maxUploadBytes, err := ParseSize(*maxUploadFlag)
	if err != nil {
		return nil, fmt.Errorf("invalid max-upload value: %w", err)
	}

	return &Config{
		Root:         absRoot,
		Port:         *portFlag,
		Host:         *hostFlag,
		ReadTimeout:  *readTimeoutFlag,
		WriteTimeout: *writeTimeoutFlag,
		MaxUpload:    maxUploadBytes,
	}, nil
}

func ParseSize(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty size")
	}

	// Find where the suffix starts
	var numStr string
	var unitStr string
	for i, r := range s {
		if (r < '0' || r > '9') && r != '.' {
			numStr = s[:i]
			unitStr = s[i:]
			break
		}
	}
	if numStr == "" {
		numStr = s
	}

	val, err := strconv.ParseFloat(strings.TrimSpace(numStr), 64)
	if err != nil {
		return 0, fmt.Errorf("invalid number format: %w", err)
	}

	unitStr = strings.ToUpper(strings.TrimSpace(unitStr))
	var multiplier int64 = 1

	switch unitStr {
	case "", "B":
		multiplier = 1
	case "KB", "K":
		multiplier = 1024
	case "MB", "M":
		multiplier = 1024 * 1024
	case "GB", "G":
		multiplier = 1024 * 1024 * 1024
	case "TB", "T":
		multiplier = 1024 * 1024 * 1024 * 1024
	default:
		return 0, fmt.Errorf("unknown unit: %s", unitStr)
	}

	return int64(val * float64(multiplier)), nil
}
