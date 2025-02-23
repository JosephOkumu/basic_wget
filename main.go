package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Config struct {
	outputFile    string
	outputDir     string
	background    bool
	rateLimit     string
	inputFile     string
	mirror        bool
	reject        string
	exclude       string
	convertLinks  bool
}

type DownloadProgress struct {
	total     int64
	current   int64
	startTime time.Time
}

func (dp *DownloadProgress) Write(p []byte) (int, error) {
	n := len(p)
	dp.current += int64(n)
	dp.printProgress()
	return n, nil
}

func (dp *DownloadProgress) printProgress() {
	if dp.total == 0 {
		return
	}
	
	percent := float64(dp.current) * 100 / float64(dp.total)
	elapsed := time.Since(dp.startTime)
	speed := float64(dp.current) / elapsed.Seconds() / 1024 // KB/s
	
	// Create progress bar
	width := 50
	completed := int(float64(width) * float64(dp.current) / float64(dp.total))
	bar := strings.Repeat("=", completed) + strings.Repeat(" ", width-completed)
	
	// Calculate remaining time
	remainingBytes := dp.total - dp.current
	remainingTime := time.Duration(float64(remainingBytes) / (float64(dp.current) / elapsed.Seconds()) * float64(time.Second))
	if dp.current == dp.total {
		remainingTime = 0
	}
	
	fmt.Printf("\r %.2f KiB / %.2f KiB [%s] %.2f%% %.2f KiB/s %v",
		float64(dp.current)/1024,
		float64(dp.total)/1024,
		bar,
		percent,
		speed,
		remainingTime.Round(time.Second))
	
	if dp.current == dp.total {
		fmt.Println()
	}
}

func downloadFile(url string, config Config) error {
	startTime := time.Now()
	fmt.Printf("start at %s\n", startTime.Format("2006-01-02 15:04:05"))

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	fmt.Printf("sending request, awaiting response... status %s\n", resp.Status)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	contentLength := resp.ContentLength
	fmt.Printf("content size: %d [~%.2fMB]\n", contentLength, float64(contentLength)/(1024*1024))

	fileName := config.outputFile
	if fileName == "" {
		fileName = filepath.Base(url)
	}
	
	if config.outputDir != "" {
		err = os.MkdirAll(config.outputDir, 0755)
		if err != nil {
			return err
		}
		fileName = filepath.Join(config.outputDir, fileName)
	}

	fmt.Printf("saving file to: %s\n", fileName)

	out, err := os.Create(fileName)
	if err != nil {
		return err
	}
	defer out.Close()

	progress := &DownloadProgress{
		total:     contentLength,
		startTime: time.Now(),
	}

	_, err = io.Copy(out, io.TeeReader(resp.Body, progress))
	if err != nil {
		return err
	}

	fmt.Printf("\nDownloaded [%s]\n", url)
	fmt.Printf("finished at %s\n", time.Now().Format("2006-01-02 15:04:05"))
	return nil
}

func downloadMultipleFiles(inputFile string, config Config) error {
	file, err := os.Open(inputFile)
	if err != nil {
		return err
	}
	defer file.Close()

	var urls []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		urls = append(urls, scanner.Text())
	}

	var wg sync.WaitGroup
	for _, url := range urls {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()
			if err := downloadFile(url, config); err != nil {
				log.Printf("Error downloading %s: %v\n", url, err)
			}
		}(url)
	}
	wg.Wait()
	return nil
}

func main() {
	config := Config{}
	
	flag.StringVar(&config.outputFile, "O", "", "Output file name")
	flag.StringVar(&config.outputDir, "P", "", "Output directory")
	flag.BoolVar(&config.background, "B", false, "Download in background")
	flag.StringVar(&config.rateLimit, "rate-limit", "", "Rate limit (e.g., 400k)")
	flag.StringVar(&config.inputFile, "i", "", "Input file containing URLs")
	flag.BoolVar(&config.mirror, "mirror", false, "Mirror website")
	flag.StringVar(&config.reject, "R", "", "Reject file types")
	flag.StringVar(&config.exclude, "X", "", "Exclude directories")
	flag.BoolVar(&config.convertLinks, "convert-links", false, "Convert links for offline viewing")
	
	flag.Parse()

	if config.background {
		logFile, err := os.Create("wget-log")
		if err != nil {
			log.Fatal(err)
		}
		defer logFile.Close()
		log.SetOutput(logFile)
		fmt.Println("Output will be written to \"wget-log\".")
	}

	args := flag.Args()
	if len(args) == 0 && config.inputFile == "" {
		fmt.Println("Please provide a URL or use -i flag with an input file")
		os.Exit(1)
	}

	if config.inputFile != "" {
		if err := downloadMultipleFiles(config.inputFile, config); err != nil {
			log.Fatal(err)
		}
		return
	}

	if err := downloadFile(args[0], config); err != nil {
		log.Fatal(err)
	}
}
