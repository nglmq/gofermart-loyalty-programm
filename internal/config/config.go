package config

import (
	"flag"
	"os"
)

var (
	RunAddr              string
	DataBaseURL          string
	AccrualSystemAddress string
)

func ParseFlags() {
	flag.StringVar(&RunAddr, "a", "localhost:8080", "address and port to run server")
	flag.StringVar(&DataBaseURL, "d", "", "postgres connection url")
	flag.StringVar(&AccrualSystemAddress, "r", "", "accrual system address")

	flag.Parse()

	envRunAddr := os.Getenv("RUN_ADDRESS")
	if envRunAddr != "" {
		RunAddr = envRunAddr
	}

	envDBConnection := os.Getenv("DATABASE_URI")
	if envDBConnection != "" {
		DataBaseURL = envDBConnection
	}

	envAccrualURL := os.Getenv("ACCRUAL_SYSTEM_ADDRESS")
	if envAccrualURL != "" {
		AccrualSystemAddress = envAccrualURL
	}
}
