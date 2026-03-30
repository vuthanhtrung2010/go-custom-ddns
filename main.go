package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

const (
	ipProviderURL = "https://api.ipify.org"
	ipFile        = "old-ip.txt"
	cfAPIURL      = "https://api.cloudflare.com/client/v4/zones"
	envFile       = "/etc/go-custom-ddns.env"
)

// --- Structs ---

type CFRecord struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	Proxied bool   `json:"proxied"`
	TTL     int    `json:"ttl"`
}

type CFResponse struct {
	Success bool       `json:"success"`
	Result  []CFRecord `json:"result"`
}

type Zone struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ZoneResponse struct {
	Success bool   `json:"success"`
	Result  []Zone `json:"result"`
}

// --- Main Execution ---

func main() {
	setup := flag.Bool("setup", false, "Run the interactive setup TUI")
	flag.Parse()

	// 1. Check if user wants to run the setup tool
	if *setup {
		runSetup()
		return
	}

	// 2. Load environment variables (allows manual runs outside systemd)
	loadEnv()

	cfToken := os.Getenv("CLOUDFLARE_API_KEY")
	cfZoneID := os.Getenv("CLOUDFLARE_ZONE_ID")

	if cfToken == "" || cfZoneID == "" {
		fmt.Printf("Error: CLOUDFLARE_API_KEY and CLOUDFLARE_ZONE_ID are not set.\n")
		fmt.Printf("Please run with sudo and the '-setup' flag to configure: sudo go-custom-ddns -setup\n")
		os.Exit(1)
	}

	// 3. Fetch current IP
	newIP := getPublicIP()
	if newIP == "" {
		fmt.Println("Error: Could not fetch current IP address.")
		os.Exit(1)
	}

	// 4. Read baseline IP
	oldIP := getOldIP()
	fmt.Printf("Old IP: %s | New IP: %s\n", oldIP, newIP)

	if oldIP == newIP {
		fmt.Println("IP has not changed. Exiting.")
		return
	}

	if oldIP == "" {
		fmt.Println("No baseline IP found. Saving current IP and exiting to establish baseline.")
		saveIP(newIP)
		return
	}

	// 5. Fetch Cloudflare records matching the old IP
	records := getRecordsWithOldIP(cfZoneID, cfToken, oldIP)
	if len(records) == 0 {
		fmt.Printf("No 'A' records found pointing to the old IP (%s).\n", oldIP)
		saveIP(newIP) // Save new IP anyway so we stop querying Cloudflare unnecessarily
		return
	}

	// 6. Update records
	allSuccess := true
	for _, rec := range records {
		fmt.Printf("Updating record %s (%s) from %s to %s...\n", rec.Name, rec.ID, oldIP, newIP)
		if updateRecord(cfZoneID, cfToken, rec, newIP) {
			fmt.Printf("Successfully updated %s!\n", rec.Name)
		} else {
			allSuccess = false
			fmt.Printf("Failed to update record %s\n", rec.Name)
		}
	}

	// 7. Save new IP to file on success
	if allSuccess {
		saveIP(newIP)
		fmt.Println("All done! old-ip.txt updated.")
	} else {
		fmt.Println("Finished with some errors. old-ip.txt was not updated to prevent sync issues.")
	}
}

// --- Setup TUI ---

func runSetup() {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("=== Go Custom DDNS Setup ===")
	fmt.Print("Enter your Cloudflare API Token: ")
	token, _ := reader.ReadString('\n')
	token = strings.TrimSpace(token)

	req, _ := http.NewRequest("GET", cfAPIURL, nil)
	req.Header.Add("Authorization", "Bearer "+token)
	req.Header.Add("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error fetching zones. Check your connection.")
		return
	}
	defer resp.Body.Close()

	var zResp ZoneResponse
	json.NewDecoder(resp.Body).Decode(&zResp)

	if !zResp.Success || len(zResp.Result) == 0 {
		fmt.Println("No zones found or invalid token. Make sure the token has 'Zone.Zone:Read' and 'Zone.DNS:Edit' permissions.")
		return
	}

	fmt.Println("\nAvailable Zones:")
	for i, zone := range zResp.Result {
		fmt.Printf("[%d] %s (ID: %s)\n", i+1, zone.Name, zone.ID)
	}

	fmt.Print("\nSelect a zone number: ")
	var choice int
	fmt.Scanln(&choice)

	if choice < 1 || choice > len(zResp.Result) {
		fmt.Println("Invalid choice.")
		return
	}

	selectedZone := zResp.Result[choice-1]

	envContent := fmt.Sprintf("CLOUDFLARE_API_KEY=%s\nCLOUDFLARE_ZONE_ID=%s\n", token, selectedZone.ID)
	err = os.WriteFile(envFile, []byte(envContent), 0600)

	if err != nil {
		fmt.Printf("Error writing config file. Did you run this with sudo? (%v)\n", err)
		return
	}

	fmt.Printf("\nSuccessfully configured for %s!\nConfig saved to %s\n", selectedZone.Name, envFile)
}

// --- Core Helper Functions ---

func loadEnv() {
	file, err := os.Open(envFile)
	if err != nil {
		return // File doesn't exist, we'll rely on system env variables instead
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if len(line) > 0 && !strings.HasPrefix(line, "#") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				os.Setenv(parts[0], parts[1])
			}
		}
	}
}

func getPublicIP() string {
	resp, err := http.Get(ipProviderURL)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return strings.TrimSpace(string(body))
}

func getOldIP() string {
	data, err := os.ReadFile(ipFile)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func saveIP(ip string) {
	err := os.WriteFile(ipFile, []byte(ip), 0644)
	if err != nil {
		fmt.Printf("Error writing to %s: %v\n", ipFile, err)
	}
}

func getRecordsWithOldIP(zoneID, token, oldIP string) []CFRecord {
	// Only fetch 'A' records that currently match the old IP
	url := fmt.Sprintf("%s/%s/dns_records?type=A&content=%s", cfAPIURL, zoneID, oldIP)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("Authorization", "Bearer "+token)
	req.Header.Add("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error fetching records from Cloudflare:", err)
		return nil
	}
	defer resp.Body.Close()

	var cfResp CFResponse
	json.NewDecoder(resp.Body).Decode(&cfResp)

	if !cfResp.Success {
		return nil
	}

	return cfResp.Result
}

func updateRecord(zoneID, token string, rec CFRecord, newIP string) bool {
	url := fmt.Sprintf("%s/%s/dns_records/%s", cfAPIURL, zoneID, rec.ID)

	rec.Content = newIP
	payloadBytes, _ := json.Marshal(rec)

	req, _ := http.NewRequest("PUT", url, bytes.NewBuffer(payloadBytes))
	req.Header.Add("Authorization", "Bearer "+token)
	req.Header.Add("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	var cfResp CFResponse
	json.NewDecoder(resp.Body).Decode(&cfResp)

	return cfResp.Success
}
