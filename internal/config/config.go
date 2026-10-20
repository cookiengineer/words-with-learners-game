package config

import (
	"flag"
	"os"
	"path/filepath"
)

type Config struct {
	Addr       string
	DataDir    string
	PublicDir  string
	DictDir    string
	Dev        bool
	VerifyGame bool
}

func Parse() Config {
	var result Config
	flag.StringVar(&result.Addr, "addr", ":8080", "HTTP listen address")
	flag.StringVar(&result.DataDir, "data", defaultDataDir(), "profile and game data directory")
	flag.StringVar(&result.PublicDir, "public", "public", "static frontend directory used with -dev")
	flag.StringVar(&result.DictDir, "dictionaries", "dictionaries", "dictionary directory used with -dev")
	flag.BoolVar(&result.Dev, "dev", false, "serve frontend and dictionaries from disk")
	flag.BoolVar(&result.VerifyGame, "verify-games", false, "replay every game on startup and report mismatches")
	flag.Parse()
	return result
}

func defaultDataDir() string {
	if dir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(dir, "words-with-learners")
	}
	return "data"
}
