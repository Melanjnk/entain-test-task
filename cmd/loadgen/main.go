// Command loadgen sends sustained HTTP load at a fixed requests-per-second rate.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

type txBody struct {
	State         string `json:"state"`
	Amount        string `json:"amount"`
	TransactionID string `json:"transactionId"`
}

func main() {
	rate := flag.Int("rate", 25, "target requests per second")
	duration := flag.Duration("duration", 2*time.Minute, "how long to sustain load")
	baseURL := flag.String("url", envOr("BASE_URL", "http://localhost:8080"), "service base URL")
	userID := flag.Uint64("user", 0, "pin all traffic to one user (1-3); 0 rotates users")
	label := flag.String("label", "", "optional name shown in the report")
	workers := flag.Int("workers", 0, "concurrent workers (0 = rate)")
	flag.Parse()

	if *rate <= 0 {
		fmt.Fprintln(os.Stderr, "rate must be positive")
		os.Exit(1)
	}
	if *workers <= 0 {
		*workers = max(*rate, 8)
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        *workers * 2,
			MaxIdleConnsPerHost: *workers * 2,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), *duration)
	defer cancel()

	fmt.Printf("Load test: %d RPS for %s -> %s\n", *rate, *duration, *baseURL)
	if *label != "" {
		fmt.Printf("Label: %s\n", *label)
	}
	if *userID > 0 {
		fmt.Printf("Hot user: %d (row-lock contention — use only for lock demo)\n", *userID)
	}
	fmt.Printf("Workers: %d | Watch Grafana: http://localhost:3000/d/entain-balance-slo\n\n", *workers)

	var (
		sent      atomic.Int64
		okCount   atomic.Int64
		errCount  atomic.Int64
		latencies []time.Duration
		latMu     sync.Mutex
		seq       atomic.Uint64
	)

	jobs := make(chan struct{}, *workers*2)
	var wg sync.WaitGroup

	for range *workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range jobs {
				if ctx.Err() != nil {
					return
				}
				lat, ok := fire(ctx, client, *baseURL, seq.Add(1), *userID)
				sent.Add(1)
				if ok {
					okCount.Add(1)
				} else {
					errCount.Add(1)
				}
				latMu.Lock()
				latencies = append(latencies, lat)
				latMu.Unlock()
			}
		}()
	}

	interval := time.Second / time.Duration(*rate)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	progress := time.NewTicker(5 * time.Second)
	defer progress.Stop()

	start := time.Now()
loop:
	for {
		select {
		case <-ctx.Done():
			break loop
		case <-progress.C:
			elapsed := time.Since(start).Truncate(time.Second)
			fmt.Printf("  ... %s elapsed | sent=%d ok=%d errors=%d\n",
				elapsed, sent.Load(), okCount.Load(), errCount.Load())
		case <-ticker.C:
			select {
			case jobs <- struct{}{}:
			default:
				errCount.Add(1)
				sent.Add(1)
			}
		}
	}

	close(jobs)
	wg.Wait()

	elapsed := time.Since(start).Seconds()
	printReport(*label, latencies, sent.Load(), okCount.Load(), errCount.Load(), elapsed, *rate)
}

func fire(ctx context.Context, client *http.Client, baseURL string, seq uint64, pinnedUser uint64) (time.Duration, bool) {
	userID := (seq % 3) + 1
	if pinnedUser > 0 {
		userID = pinnedUser
	}
	state := "win"
	if seq%5 == 0 {
		state = "lose"
	}

	body, err := json.Marshal(txBody{
		State:         state,
		Amount:        "1.15",
		TransactionID: fmt.Sprintf("load-%d-%d", time.Now().UnixNano(), seq),
	})
	if err != nil {
		return 0, false
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/user/%d/transaction", baseURL, userID), bytes.NewReader(body))
	if err != nil {
		return 0, false
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Source-Type", pickSource(seq))

	start := time.Now()
	resp, err := client.Do(req)
	lat := time.Since(start)
	if err != nil {
		return lat, false
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	return lat, resp.StatusCode == http.StatusOK
}

func pickSource(seq uint64) string {
	switch seq % 3 {
	case 0:
		return "game"
	case 1:
		return "server"
	default:
		return "payment"
	}
}

func printReport(label string, latencies []time.Duration, sent, ok, errs int64, elapsed float64, targetRPS int) {
	actualRPS := float64(sent) / elapsed
	successRate := float64(ok) / float64(max(sent, 1)) * 100

	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })

	var p50, p95, p99, maxLat time.Duration
	if len(latencies) > 0 {
		p50 = latencies[len(latencies)*50/100]
		p95 = latencies[len(latencies)*95/100]
		p99 = latencies[len(latencies)*99/100]
		maxLat = latencies[len(latencies)-1]
	}

	fmt.Println("--- results ---")
	if label != "" {
		fmt.Printf("Label:          %s\n", label)
	}
	fmt.Printf("Target RPS:     %d\n", targetRPS)
	fmt.Printf("Actual RPS:     %.1f\n", actualRPS)
	fmt.Printf("Requests:       %d (ok=%d, errors=%d)\n", sent, ok, errs)
	fmt.Printf("Success rate:   %.2f%%\n", successRate)
	if len(latencies) > 0 {
		fmt.Printf("Latency p50:    %s\n", p50)
		fmt.Printf("Latency p95:    %s\n", p95)
		fmt.Printf("Latency p99:    %s\n", p99)
		fmt.Printf("Latency max:    %s\n", maxLat)
	}

	fmt.Printf("Verdict:        %s\n", verdict(targetRPS, actualRPS, successRate, p95))
	fmt.Println()
	printVerdictHint(targetRPS)
}

func verdict(targetRPS int, actualRPS, successRate float64, p95 time.Duration) string {
	p95Ms := p95.Seconds() * 1000
	keepsPace := actualRPS >= float64(targetRPS)*0.9

	if targetRPS <= 40 {
		if successRate >= 99 && p95Ms < 100 && keepsPace {
			return "WITHIN_SLO"
		}
		return "BELOW_SLO"
	}

	switch {
	case successRate >= 99 && p95Ms < 100 && keepsPace:
		return "HEALTHY_AT_STRESS"
	case successRate >= 95 && (p95Ms < 500 || keepsPace):
		return "GRACEFUL_DEGRADATION"
	default:
		return "SATURATED"
	}
}

func printVerdictHint(targetRPS int) {
	switch {
	case targetRPS <= 30:
		fmt.Println("Task requirement (20–30 RPS): expect WITHIN_SLO.")
	case targetRPS <= 40:
		fmt.Println("Headroom (40 RPS): still WITHIN_SLO — valid load, fast responses.")
	default:
		fmt.Println("Stress (hot user): rising p95 = GRACEFUL_DEGRADATION; errors/timeouts = SATURATED.")
		fmt.Println("Spread 200 RPS across 3 users is easy — use load-200 (hot user) or load-500.")
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
