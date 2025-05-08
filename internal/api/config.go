package api

import (
	"log"
	"os"
)

type Config struct {
	Namespace string
	Port      string
}

func GetConfigOrDie() Config {
	namespace := os.Getenv("NAMESPACE")
	if namespace == "" {
		log.Fatal("NAMESPACE environment variable is required")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080" // Default port
	}

	return Config{
		Namespace: namespace,
		Port:      port,
	}
}
