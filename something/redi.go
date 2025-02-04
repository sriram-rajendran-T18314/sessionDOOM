package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/go-redis/redis/v8"
)

func main() {
	// Define flags for Redis connection parameters and file paths.
	host := flag.String("host", "localhost", "Redis server host")
	port := flag.String("port", "6379", "Redis server port")
	username := flag.String("username", "", "Redis username (if required)")
	password := flag.String("password", "", "Redis password (if required)")
	keysFile := flag.String("keysfile", "keys.txt", "Path to file containing base64-encoded keys")
	outFile := flag.String("outfile", "output.txt", "Path to file for output")
	flag.Parse()

	// Combine host and port for the address.
	addr := fmt.Sprintf("%s:%s", *host, *port)

	// Create a Redis client.
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Username: *username,
		Password: *password,
	})

	// Create a context for Redis commands.
	ctx := context.Background()

	o := rdb.Ping(ctx)
	fmt.Println(o)

	// Prepare a slice to hold the output lines.
	var outputLines []string

	// Open the file that contains the keys.
	file, err := os.Open(*keysFile)
	if err != nil {
		log.Fatalf("Error opening file '%s': %v", *keysFile, err)
	}
	defer file.Close()

	// Create a scanner to read the file line by line.
	scanner := bufio.NewScanner(file)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		// Read the line and trim whitespace.
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue // Skip empty lines.
		}
		// Remove surrounding quotes, if present.
		encodedKey := strings.Trim(line, `"`)

		// Base64 decode the key.
		decodedBytes, err := base64.StdEncoding.DecodeString(encodedKey)
		if err != nil {
			log.Printf("Line %d: error decoding base64 key '%s': %v", lineNumber, encodedKey, err)
			continue
		}
		decodedKey := string(decodedBytes)

		// Construct the Redis key.
		// For example: "spring:session:sessions:" + decodedKey + "sessionAttr:DEVICE_TYPE"
		rk := "spring:session:sessions:" + decodedKey + " sessionAttr:DEVICE_TYPE"

		// Retrieve the value from Redis using GET.
		val, err := rdb.Get(ctx, rk).Result()
		if err == redis.Nil {
			log.Printf("Line %d: key '%s' does not exist in Redis", lineNumber, decodedKey)
			continue
		} else if err != nil {
			log.Printf("Line %d: error retrieving key '%s': %v", lineNumber, rk, err)
			continue
		}

		// Format the output.
		outputLine := fmt.Sprintf("Key: %s, Value: %s", decodedKey, val)
		outputLines = append(outputLines, outputLine)
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("Error reading keys file: %v", err)
	}

	// Write the output lines to the output file.
	outputContent := strings.Join(outputLines, "\n")
	err = os.WriteFile(*outFile, []byte(outputContent), 0644)
	if err != nil {
		log.Fatalf("Error writing output to file '%s': %v", *outFile, err)
	}

	fmt.Printf("Output successfully written to %s\n", *outFile)
}
