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

// LoadEnv loads environment variables from a .env file if it exists.
// It does not overwrite existing environment variables.
func LoadEnv(filename string) error {
	content, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // .env is optional
		}
		return err
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		// Handle quotes and inline comments
		if strings.HasPrefix(val, "\"") {
			if idx := strings.Index(val[1:], "\""); idx != -1 {
				val = val[1 : idx+1]
			}
		} else if strings.HasPrefix(val, "'") {
			if idx := strings.Index(val[1:], "'"); idx != -1 {
				val = val[1 : idx+1]
			}
		} else {
			if idx := strings.Index(val, "#"); idx != -1 {
				val = strings.TrimSpace(val[:idx])
			}
		}

		// Only set if not already set in the current process
		if os.Getenv(key) == "" {
			if err := os.Setenv(key, val); err != nil {
				return err
			}
		}
	}
	return nil
}

func ParseConfig() (*Config, error) {
	// 1. Load .env first if it exists
	if err := LoadEnv(".env"); err != nil {
		return nil, fmt.Errorf("failed to load .env: %w", err)
	}

	// 2. Define flags
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

	// Track which flags were set on the command line
	setFlags := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) {
		setFlags[f.Name] = true
	})

	// Override unset flags with environment variables if present
	var root string
	if setFlags["root"] {
		root = *rootFlag
	} else if envRoot := os.Getenv("GOPHRDRV_ROOT"); envRoot != "" {
		root = envRoot
	} else {
		root = *rootFlag
	}

	var port int
	if setFlags["port"] {
		port = *portFlag
	} else if envPort := os.Getenv("GOPHRDRV_PORT"); envPort != "" {
		p, err := strconv.Atoi(envPort)
		if err != nil {
			return nil, fmt.Errorf("invalid GOPHRDRV_PORT value %q: %w", envPort, err)
		}
		port = p
	} else {
		port = *portFlag
	}

	var host string
	if setFlags["host"] {
		host = *hostFlag
	} else if envHost := os.Getenv("GOPHRDRV_HOST"); envHost != "" {
		host = envHost
	} else {
		host = *hostFlag
	}

	var readTimeout time.Duration
	if setFlags["read-timeout"] {
		readTimeout = *readTimeoutFlag
	} else if envReadTimeout := os.Getenv("GOPHRDRV_READ_TIMEOUT"); envReadTimeout != "" {
		d, err := time.ParseDuration(envReadTimeout)
		if err != nil {
			return nil, fmt.Errorf("invalid GOPHRDRV_READ_TIMEOUT value %q: %w", envReadTimeout, err)
		}
		readTimeout = d
	} else {
		readTimeout = *readTimeoutFlag
	}

	var writeTimeout time.Duration
	if setFlags["write-timeout"] {
		writeTimeout = *writeTimeoutFlag
	} else if envWriteTimeout := os.Getenv("GOPHRDRV_WRITE_TIMEOUT"); envWriteTimeout != "" {
		d, err := time.ParseDuration(envWriteTimeout)
		if err != nil {
			return nil, fmt.Errorf("invalid GOPHRDRV_WRITE_TIMEOUT value %q: %w", envWriteTimeout, err)
		}
		writeTimeout = d
	} else {
		writeTimeout = *writeTimeoutFlag
	}

	var maxUploadStr string
	if setFlags["max-upload"] {
		maxUploadStr = *maxUploadFlag
	} else if envMaxUpload := os.Getenv("GOPHRDRV_MAX_UPLOAD"); envMaxUpload != "" {
		maxUploadStr = envMaxUpload
	} else {
		maxUploadStr = *maxUploadFlag
	}

	// Handle root directory
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

	maxUploadBytes, err := ParseSize(maxUploadStr)
	if err != nil {
		return nil, fmt.Errorf("invalid max-upload value: %w", err)
	}

	return &Config{
		Root:         absRoot,
		Port:         port,
		Host:         host,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
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
