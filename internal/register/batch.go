package register

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/exagen-creator/exagen/internal/email"
)

// registerOne handles a single account registration.
func registerOne(workerID int, tag string, proxy, outputFile, defaultDomain string, printMu, fileMu *sync.Mutex) (bool, string, string) {
	client, err := NewClient(proxy, tag, workerID, printMu, fileMu)
	if err != nil {
		return false, "", fmt.Sprintf("failed to create client: %v", err)
	}

	inbox, err := email.CreateTempEmail(defaultDomain)
	if err != nil {
		return false, "", fmt.Sprintf("failed to create temp email: %v", err)
	}

	emailAddr := inbox.Address
	client.print(fmt.Sprintf("Starting registration for %s", emailAddr))

	err = client.RunRegister(emailAddr, inbox)
	if err != nil {
		return false, emailAddr, err.Error()
	}

	// Retrieve API key after registration
	client.randomDelay(0.5, 1.0)
	apiKey, err := client.GetAPIKey()
	if err != nil {
		client.print(fmt.Sprintf("Warning: failed to get API key: %v", err))
		apiKey = ""
	}

	// Append to file: email|apiKey
	fileMu.Lock()
	defer fileMu.Unlock()

	f, err := os.OpenFile(outputFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return false, emailAddr, fmt.Sprintf("failed to open output file: %v", err)
	}
	defer f.Close()

	line := fmt.Sprintf("%s|%s\n", emailAddr, apiKey)
	if _, err := f.WriteString(line); err != nil {
		return false, emailAddr, fmt.Sprintf("failed to write to output file: %v", err)
	}

	return true, emailAddr, ""
}

// RunBatch runs concurrent registration tasks with retry until target success count is reached.
func RunBatch(totalAccounts int, outputFile string, maxWorkers int, proxy, defaultDomain string) {
	const maxRetries = 3

	var printMu sync.Mutex
	var fileMu sync.Mutex

	var remaining int64 = int64(totalAccounts)
	var successCount int64
	var failureCount int64

	startTime := time.Now()

	var wg sync.WaitGroup

	for w := 1; w <= maxWorkers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for {
				if atomic.AddInt64(&remaining, -1) < 0 {
					atomic.AddInt64(&remaining, 1)
					return
				}

				cur := atomic.LoadInt64(&successCount) + 1
				tag := fmt.Sprintf("%d/%d", cur, totalAccounts)

				var success bool
				var emailAddr, errStr string
				for retry := 0; retry <= maxRetries; retry++ {
					if retry > 0 {
						printMu.Lock()
						ts := time.Now().Format("15:04:05")
						fmt.Printf("[%s] [W%d] Retry %d/%d for slot %s\n", ts, workerID, retry, maxRetries, tag)
						printMu.Unlock()
						time.Sleep(time.Duration(retry) * 2 * time.Second)
					}

					success, emailAddr, errStr = registerOne(workerID, tag, proxy, outputFile, defaultDomain, &printMu, &fileMu)
					if success {
						break
					}
				}

				if success {
					atomic.AddInt64(&successCount, 1)
					ts := time.Now().Format("15:04:05")
					printMu.Lock()
					fmt.Printf("[%s] [W%d] SUCCESS: %s\n", ts, workerID, emailAddr)
					printMu.Unlock()
				} else {
					atomic.AddInt64(&failureCount, 1)
					atomic.AddInt64(&remaining, 1)
					ts := time.Now().Format("15:04:05")

					printMu.Lock()
					fmt.Printf("[%s] [W%d] FAILED after %d retries: %s | %s\n", ts, workerID, maxRetries, emailAddr, errStr)
					printMu.Unlock()
				}
			}
		}(w)
	}

	wg.Wait()

	elapsed := time.Since(startTime)
	elapsedStr := formatDuration(elapsed)

	fmt.Printf("\n--- Batch Registration Summary ---\n")
	fmt.Printf("Target:    %d\n", totalAccounts)
	fmt.Printf("Success:   %d\n", successCount)
	fmt.Printf("Failures:  %d\n", failureCount)
	fmt.Printf("Elapsed:   %s\n", elapsedStr)
	fmt.Printf("Output:    %s (email|apiKey)\n", outputFile)
	fmt.Printf("----------------------------------\n")
}

func formatDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60

	if h > 0 {
		return fmt.Sprintf("%dh %dm %ds", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}
