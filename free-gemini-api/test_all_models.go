//go:build ignore

package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"goapi/gemini"
)

type ModelTestStatus int

const (
	StatusPending ModelTestStatus = iota
	StatusRunning
	StatusOK
	StatusFallback
	StatusSubRequired
	StatusError
)

type ModelTestItem struct {
	Index        int
	ID           string
	ModeID       string
	ThinkingLvl  int
	DisplayName  string
	Status       ModelTestStatus
	StartTime    time.Time
	Duration     time.Duration
	ActualModel  string
	Fallback     bool
	ResponseText string
	ErrorMsg     string
}

func main() {
	defaultPrompt := "Rispondi in una parola quale è la capitale della Francia"

	promptFlag := flag.String("prompt", "", "Prompt da inviare a tutti i modelli")
	serverFlag := flag.String("server", "http://127.0.0.1:8001", "URL del server API locale")
	concurrencyFlag := flag.Int("concurrency", 4, "Numero massimo di richieste parallele simultanee (0 = tutte insieme)")
	flag.Parse()

	prompt := defaultPrompt
	if *promptFlag != "" {
		prompt = *promptFlag
	} else if len(flag.Args()) > 0 {
		prompt = strings.Join(flag.Args(), " ")
	}

	// Recupera i modelli definiti in DefaultBuiltinModels
	models := gemini.DefaultBuiltinModels
	if len(models) == 0 {
		models = gemini.GetModelRegistry().GetAll()
	}

	// Verifica se il server è raggiungibile
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(*serverFlag + "/v1/models")
	if err != nil {
		fmt.Printf("\033[1;31m❌ Impossibile connettersi al server API su %s\033[0m\n", *serverFlag)
		fmt.Printf("Assicurati che il server sia avviato con: \033[1;32mgo run main.go --temp\033[0m\n\n")
		os.Exit(1)
	}
	resp.Body.Close()

	items := make([]*ModelTestItem, len(models))
	for i, m := range models {
		items[i] = &ModelTestItem{
			Index:       i + 1,
			ID:          m.ID,
			ModeID:      m.ModeID,
			ThinkingLvl: m.ThinkingLevel,
			DisplayName: m.DisplayName,
			Status:      StatusPending,
		}
	}

	var mu sync.Mutex
	doneChan := make(chan struct{})
	var wg sync.WaitGroup

	// Concurrency semaphore
	maxConc := *concurrencyFlag
	var sem chan struct{}
	if maxConc > 0 {
		sem = make(chan struct{}, maxConc)
	}

	// Trap SIGINT per ripristinare il terminale
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		mu.Lock()
		fmt.Print("\033[?25h\n\nTest interrotto dall'utente.\n")
		mu.Unlock()
		os.Exit(0)
	}()

	// Nascondi cursore terminale durante l'aggiornamento
	fmt.Print("\033[?25l")
	defer fmt.Print("\033[?25h\n")

	startTime := time.Now()

	// Goroutine per rendering della tabella in tempo reale
	go func() {
		animFrames := []string{".", "..", "...", "....", "..."}
		frameIdx := 0
		ticker := time.NewTicker(150 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-doneChan:
				renderTable(items, prompt, startTime, animFrames[frameIdx%len(animFrames)], true)
				return
			case <-ticker.C:
				frameIdx++
				mu.Lock()
				renderTable(items, prompt, startTime, animFrames[frameIdx%len(animFrames)], false)
				mu.Unlock()
			}
		}
	}()

	// Esecuzione parallela
	httpClient := &http.Client{Timeout: 120 * time.Second}
	endpoint := *serverFlag + "/v1/chat/completions"

	for _, item := range items {
		wg.Add(1)
		go func(it *ModelTestItem) {
			defer wg.Done()

			if sem != nil {
				sem <- struct{}{}
				defer func() { <-sem }()
			}

			mu.Lock()
			it.Status = StatusRunning
			it.StartTime = time.Now()
			mu.Unlock()

			reqBody := map[string]interface{}{
				"model": it.ID,
				"messages": []map[string]string{
					{"role": "user", "content": prompt},
				},
			}
			jsonBytes, _ := json.Marshal(reqBody)

			httpReq, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(jsonBytes))
			if err != nil {
				mu.Lock()
				it.Status = StatusError
				it.Duration = time.Since(it.StartTime)
				it.ErrorMsg = err.Error()
				mu.Unlock()
				return
			}
			httpReq.Header.Set("Content-Type", "application/json")

			httpResp, err := httpClient.Do(httpReq)
			elapsed := time.Since(it.StartTime)

			mu.Lock()
			defer mu.Unlock()
			it.Duration = elapsed

			if err != nil {
				it.Status = StatusError
				it.ErrorMsg = "Timeout / Connessione fallita"
				return
			}
			defer httpResp.Body.Close()

			bodyBytes, _ := io.ReadAll(httpResp.Body)

			actualModel := httpResp.Header.Get("X-Actual-Model")
			fallbackHeader := httpResp.Header.Get("X-Fallback-Detected")

			it.ActualModel = actualModel
			if fallbackHeader == "true" {
				it.Fallback = true
			}

			if httpResp.StatusCode != 200 {
				var errResp struct {
					Error struct {
						Message string `json:"message"`
					} `json:"error"`
					Err string `json:"error"`
				}
				json.Unmarshal(bodyBytes, &errResp)
				msg := errResp.Error.Message
				if msg == "" {
					msg = errResp.Err
				}
				if msg == "" {
					msg = fmt.Sprintf("HTTP %d", httpResp.StatusCode)
				}
				it.ErrorMsg = truncate(msg, 32)
				if httpResp.StatusCode == 403 {
					it.Status = StatusSubRequired
				} else {
					it.Status = StatusError
				}
				return
			}

			var compResp struct {
				Choices []struct {
					Message struct {
						Content string `json:"content"`
					} `json:"message"`
				} `json:"choices"`
			}
			if err := json.Unmarshal(bodyBytes, &compResp); err != nil || len(compResp.Choices) == 0 {
				it.Status = StatusError
				it.ErrorMsg = "Risposta non valida"
				return
			}

			reply := strings.TrimSpace(compResp.Choices[0].Message.Content)
			reply = strings.ReplaceAll(reply, "\n", " ")
			it.ResponseText = reply

			if it.Fallback || gemini.IsModelDowngraded(it.ID, actualModel) {
				it.Status = StatusFallback
			} else {
				it.Status = StatusOK
			}
		}(item)
	}

	wg.Wait()
	close(doneChan)
	time.Sleep(200 * time.Millisecond)
}

func renderTable(items []*ModelTestItem, prompt string, start time.Time, dots string, final bool) {
	var b strings.Builder

	// ANSI: Move cursor to home (top left)
	b.WriteString("\033[H")

	// Header
	b.WriteString("\033[1;36m═══════════════════════════════════════════════════════════════════════════════════════════════════════════════════\033[0m\n")
	b.WriteString(fmt.Sprintf("\033[1;37m 🚀 GEMINI MODELS LIVE PARALLEL TEST SUITE \033[0m (Totale Modelli: \033[1;33m%d\033[0m | Tempo: \033[1;32m%.1fs\033[0m)\n", len(items), time.Since(start).Seconds()))
	b.WriteString(fmt.Sprintf("\033[1;30m 💬 Prompt: \033[0;37m\"%s\"\033[0m\n", truncate(prompt, 85)))
	b.WriteString("\033[1;36m───────────────────────────────────────────────────────────────────────────────────────────────────────────\033[0m\n")

	// Table column headers
	b.WriteString(fmt.Sprintf("\033[1m %-3s │ %-34s │ %-16s │ %-10s │ %-7s │ %-32s \033[0m\n",
		"#", "MODEL ID", "MODE ID", "STATUS", "TEMPO", "RISPOSTA / STATO"))
	b.WriteString("\033[1;36m─────┼────────────────────────────────────┼──────────────────┼────────────┼─────────┼─────────────────────────────────\033[0m\n")

	okCount := 0
	fallbackCount := 0
	subCount := 0
	errCount := 0
	runningCount := 0

	for _, it := range items {
		var statusStr string
		var timeStr string
		var respStr string

		switch it.Status {
		case StatusPending:
			statusStr = "\033[0;37mIn attesa\033[0m"
			timeStr = "—"
			respStr = "\033[0;30mIn coda...\033[0m"
		case StatusRunning:
			runningCount++
			statusStr = fmt.Sprintf("\033[1;33mRunning %-3s\033[0m", dots)
			timeStr = fmt.Sprintf("\033[0;33m%5.1fs\033[0m", time.Since(it.StartTime).Seconds())
			respStr = fmt.Sprintf("\033[0;33mIn attesa di risposta %s\033[0m", dots)
		case StatusOK:
			okCount++
			statusStr = "\033[1;32m✅ 200 OK \033[0m"
			timeStr = fmt.Sprintf("\033[0;32m%5.1fs\033[0m", it.Duration.Seconds())
			respStr = fmt.Sprintf("\033[0;32m%s\033[0m", truncate(it.ResponseText, 32))
		case StatusFallback:
			fallbackCount++
			statusStr = "\033[1;31m⚠️ FALLBACK\033[0m"
			timeStr = fmt.Sprintf("\033[0;31m%5.1fs\033[0m", it.Duration.Seconds())
			detail := it.ActualModel
			if detail == "" {
				detail = "Segnale 37"
			}
			respStr = fmt.Sprintf("\033[1;31m[%s] %s\033[0m", truncate(detail, 10), truncate(it.ResponseText, 20))
		case StatusSubRequired:
			subCount++
			statusStr = "\033[1;35m🔒 SUB REQ\033[0m"
			timeStr = fmt.Sprintf("\033[0;35m%5.1fs\033[0m", it.Duration.Seconds())
			respStr = fmt.Sprintf("\033[0;35m%s\033[0m", truncate(it.ErrorMsg, 32))
		case StatusError:
			errCount++
			statusStr = "\033[1;31m❌ ERROR  \033[0m"
			timeStr = fmt.Sprintf("\033[0;31m%5.1fs\033[0m", it.Duration.Seconds())
			respStr = fmt.Sprintf("\033[1;31m%s\033[0m", truncate(it.ErrorMsg, 32))
		}

		b.WriteString(fmt.Sprintf(" %-3d │ %-34s │ %-16s │ %-19s │ %-7s │ %-32s\n",
			it.Index, truncate(it.ID, 34), truncate(it.ModeID, 16), statusStr, timeStr, respStr))
	}

	b.WriteString("\033[1;36m═══════════════════════════════════════════════════════════════════════════════════════════════════════════════════\033[0m\n")
	b.WriteString(fmt.Sprintf(" \033[1mRIEPILOGO:\033[0m  \033[1;32m✅ OK: %d\033[0m  │  \033[1;35m🔒 Sub Req: %d\033[0m  │  \033[1;31m⚠️ Fallback: %d\033[0m  │  \033[1;31m❌ Errori: %d\033[0m  │  \033[1;33m⏳ In corso: %d\033[0m\n",
		okCount, subCount, fallbackCount, errCount, runningCount))

	if final {
		b.WriteString("\033[1;32m ✨ Test di tutti i modelli completato con successo!\033[0m\n\n")
	}

	fmt.Print(b.String())
}

func truncate(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return string(runes[:maxLen])
	}
	return string(runes[:maxLen-3]) + "..."
}
