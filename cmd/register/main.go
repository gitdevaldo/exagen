package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/exagen-creator/exagen/internal/config"
	"github.com/exagen-creator/exagen/internal/register"
)

func main() {
	printBanner()

	cfg, err := config.Load("config.json")
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	reader := bufio.NewReader(os.Stdin)

	// 1. Proxy prompt
	proxy := cfg.Proxy
	if cfg.Proxy == "" {
		fmt.Printf("Proxy (enter to skip): ")
		proxyInput, _ := reader.ReadString('\n')
		proxy = strings.TrimSpace(proxyInput)
	}

	// 2. Total accounts prompt
	fmt.Printf("Total accounts to register: ")
	totalInput, _ := reader.ReadString('\n')
	totalInput = strings.TrimSpace(totalInput)

	if totalInput == "" {
		fmt.Println("Error: total accounts is required.")
		os.Exit(1)
	}
	totalAccounts, err := strconv.Atoi(totalInput)
	if err != nil {
		fmt.Printf("Error: invalid number '%s'.\n", totalInput)
		os.Exit(1)
	}

	// 3. Max workers prompt
	defaultWorkers := 3
	fmt.Printf("Max concurrent workers (default: %d): ", defaultWorkers)
	workersInput, _ := reader.ReadString('\n')
	workersInput = strings.TrimSpace(workersInput)

	maxWorkers := defaultWorkers
	if workersInput != "" {
		if val, err := strconv.Atoi(workersInput); err == nil {
			maxWorkers = val
		}
	}

	// 4. Default domain prompt
	defaultDomain := cfg.DefaultDomain
	if cfg.DefaultDomain == "" {
		fmt.Printf("Default domain (current: (random from generator.email), press Enter to use, or enter new): ")
		domainInput, _ := reader.ReadString('\n')
		domainInput = strings.TrimSpace(domainInput)

		if domainInput != "" {
			defaultDomain = domainInput
		}
	}

	fmt.Println()
	fmt.Println("-------------------------------------------")
	fmt.Printf("Configuration:\n")
	fmt.Printf("  Proxy:          %s\n", proxy)
	fmt.Printf("  Total Accounts: %d\n", totalAccounts)
	fmt.Printf("  Max Workers:    %d\n", maxWorkers)
	if defaultDomain != "" {
		fmt.Printf("  Domain:         %s\n", defaultDomain)
	} else {
		fmt.Printf("  Domain:         (random)\n")
	}
	if cfg.VcrcsCookie != "" {
		fmt.Printf("  Vercel Cookie:  %s...%s\n", cfg.VcrcsCookie[:10], cfg.VcrcsCookie[len(cfg.VcrcsCookie)-10:])
	}
	fmt.Printf("  Output File:    %s\n", cfg.OutputFile)
	fmt.Println("-------------------------------------------")
	fmt.Println()

	register.RunBatch(totalAccounts, cfg.OutputFile, maxWorkers, proxy, defaultDomain, cfg.VcrcsCookie)
}

func printBanner() {
	banner := `
  ______            
 |  ____|           
 | |__  __  ____ _  
 |  __| \ \/ / _` + "`" + ` | 
 | |____ >  < (_| | 
 |______/_/\_\__,_| 

     Exa Account Registration Bot
`
	fmt.Println(banner)
}
