package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"log"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func handleHome(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if _, err := os.Stat("index.html"); os.IsNotExist(err) {
		_, _ = w.Write(indexBytes)
	} else {
		var bs []byte
		if bs, err = os.ReadFile("index.html"); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_, _ = w.Write(bs)
	}
}

func handleDownload(w http.ResponseWriter, r *http.Request) {
	contentType := r.Header.Get("Content-Type")
	if !strings.Contains(contentType, "multipart/form-data") {
		http.Error(w,
			fmt.Sprintf(
				"The Content-Type: %s is not supported. This API only accepts: 'multipart/form-data'",
				contentType,
			),
			http.StatusUnsupportedMediaType,
		)
		return
	}

	if err := r.ParseMultipartForm(int64(1 << 20)); err != nil { // 1 MB
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	targetURL := r.PostForm.Get("url")
	if targetURL == "" || !strings.HasPrefix(targetURL, "http") {
		http.Error(w, "The URL does not start with http or empty", http.StatusBadRequest)
		return
	}

	result := struct {
		Status int    `json:"status"`
		Msg    string `json:"msg,omitempty"`
	}{}

	filename := r.PostForm.Get("filename")

	displayHeaderFooter := r.PostForm.Get("displayHeaderFooter") == "on"
	printBackground := r.PostForm.Get("printBackground") == "on"

	width, err1 := strconv.ParseFloat(r.PostForm.Get("width"), 64)
	height, err2 := strconv.ParseFloat(r.PostForm.Get("height"), 64)
	if errBorder := errors.Join(err1, err2); errBorder != nil {
		http.Error(w, "size error"+errBorder.Error(), http.StatusBadRequest)
		return
	}

	marginTop, err1 := strconv.ParseFloat(r.PostForm.Get("top"), 64)
	marginBottom, err2 := strconv.ParseFloat(r.PostForm.Get("bottom"), 64)
	marginLeft, err3 := strconv.ParseFloat(r.PostForm.Get("left"), 64)
	marginRight, err4 := strconv.ParseFloat(r.PostForm.Get("right"), 64)
	if errBorder := errors.Join(err1, err2, err3, err4); errBorder != nil {
		http.Error(w, "margin error"+errBorder.Error(), http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	opts := chromedp.DefaultExecAllocatorOptions[:]

	if r.PostForm.Get("headless") == "on" {
		opts = append(opts, []chromedp.ExecAllocatorOption{
			chromedp.Flag("headless", false),
			chromedp.Flag("start-maximized", true),
		}...)
	}

	allocCtx, cancel2 := chromedp.NewExecAllocator(ctx, opts...)
	defer cancel2()

	var ctx2 context.Context
	var cancel3 context.CancelFunc
	if r.PostForm.Get("debug") == "on" {
		ctx2, cancel3 = chromedp.NewContext(allocCtx, chromedp.WithDebugf(log.Printf))
	} else {
		ctx2, cancel3 = chromedp.NewContext(allocCtx)
	}
	defer cancel3()

	tasks := chromedp.Tasks{
		chromedp.Navigate(targetURL),
	}

	/*
		waitVisible := r.PostForm.Get("waitVisible")
		if waitVisible != "" {
			tasks = append(tasks, chromedp.WaitVisible(waitVisible))
		}
	*/
	if len(r.PostForm.Get("sleep")) > 0 {
		sleep, err := strconv.ParseInt(r.PostForm.Get("sleep"), 10, 64)
		if err == nil {
			tasks = append(tasks, chromedp.Sleep(time.Duration(sleep)*time.Second))
		}
	}

	tasks = append(tasks, chromedp.ActionFunc(func(ctx context.Context) error {
		f, err := os.Create(filename)
		if err != nil {
			return err
		}
		defer func() {
			_ = f.Close()
		}()
		var buf []byte
		buf, _, err = page.PrintToPDF().
			WithDisplayHeaderFooter(displayHeaderFooter).
			WithPrintBackground(printBackground). // 建議啟用

			// a4 8.3 x 11.7
			WithPaperWidth(width).
			WithPaperHeight(height).

			// 邊界設定為0
			WithMarginTop(marginTop).
			WithMarginBottom(marginBottom).
			WithMarginLeft(marginLeft).
			WithMarginRight(marginRight).
			Do(ctx)
		if err != nil {
			return err
		}
		_, err = f.Write(buf)
		return err
	}))

	if err := chromedp.Run(ctx2, tasks); err != nil {
		result.Status = http.StatusInternalServerError
		result.Msg = err.Error()
	} else {
		result.Status = http.StatusOK
		result.Msg = fmt.Sprintf("file created: %s | %s", filename, time.Now().Format("2006/01/02 15:04:05"))
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(&result)
}

//go:embed index.html
var indexBytes []byte

func main() {
	var port int
	flag.IntVar(&port, "port", 9000, "port number")
	flag.Parse()

	http.HandleFunc("GET /", handleHome)
	http.HandleFunc("POST /download", handleDownload)

	serverURL := "http://127.0.0.1:" + strconv.Itoa(port)
	go func() {
		// https://stackoverflow.com/a/39324149/9935654
		<-time.After(100 * time.Millisecond) // wait server start
		var cmd string
		switch runtime.GOOS {
		case "darwin":
			cmd = "open"
		case "windows":
			cmd = "explorer"
		default: // "linux", "freebsd", "openbsd", "netbsd"
			cmd = "xdg-open"
		}
		_ = exec.Command(cmd, serverURL).Start()
	}()

	fmt.Println("Please visit the webpage: ", serverURL)
	if err := http.ListenAndServe(strings.Trim(serverURL, "http://"), nil); err != nil {
		log.Fatal(err)
	}

}
